package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store backed by a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies migrations.
func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) AppendReceipt(r vur.Receipt) (int64, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(`INSERT INTO receipts (receipt_json) VALUES (?)`, string(b))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetReceiptByID(id string) (*StoredReceipt, error) {
	row := s.db.QueryRow(
		`SELECT id, receipt_json FROM receipts WHERE json_extract(receipt_json,'$.id')=? LIMIT 1`, id)
	var rowID int64
	var js string
	if err := row.Scan(&rowID, &js); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var r vur.Receipt
	if err := json.Unmarshal([]byte(js), &r); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	return &StoredReceipt{RowID: rowID, Receipt: r}, nil
}

func (s *SQLiteStore) FetchUnbatched(limit int) ([]StoredReceipt, error) {
	rows, err := s.db.Query(
		`SELECT id, receipt_json FROM receipts WHERE batched=0 ORDER BY id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoredReceipts(rows)
}

func (s *SQLiteStore) MarkBatched(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.db.Exec(`UPDATE receipts SET batched=1 WHERE id IN (`+placeholders+`)`, args...)
	return err
}

func (s *SQLiteStore) GetReceiptsByRange(startID, endID int64) ([]vur.Receipt, error) {
	rows, err := s.db.Query(
		`SELECT id, receipt_json FROM receipts WHERE id >= ? AND id <= ? ORDER BY id ASC`,
		startID, endID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReceipts(rows)
}

func (s *SQLiteStore) SaveCheckpoint(c Checkpoint) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO checkpoints
		 (batch_start, batch_end, tree_size, root_hex, previous_root_hex, issued_at, operator_sig_hex)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.BatchStart, c.BatchEnd, c.TreeSize, c.RootHex, c.PreviousRootHex,
		c.IssuedAt.UTC().Unix(), c.OperatorSigHex,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetCheckpoint(id int64) (*Checkpoint, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_start, batch_end, tree_size, root_hex, previous_root_hex,
		        issued_at, operator_sig_hex, rekor_log_index
		 FROM checkpoints WHERE id=?`, id)
	return scanCheckpoint(row)
}

func (s *SQLiteStore) ListCheckpoints(offset, limit int) ([]Checkpoint, error) {
	rows, err := s.db.Query(
		`SELECT id, batch_start, batch_end, tree_size, root_hex, previous_root_hex,
		        issued_at, operator_sig_hex, rekor_log_index
		 FROM checkpoints ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cps []Checkpoint
	for rows.Next() {
		cp, err := scanCheckpointRow(rows)
		if err != nil {
			return nil, err
		}
		cps = append(cps, *cp)
	}
	return cps, rows.Err()
}

func (s *SQLiteStore) LatestCheckpoint() (*Checkpoint, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_start, batch_end, tree_size, root_hex, previous_root_hex,
		        issued_at, operator_sig_hex, rekor_log_index
		 FROM checkpoints ORDER BY id DESC LIMIT 1`)
	return scanCheckpoint(row)
}

func (s *SQLiteStore) UpdateCheckpointRekorIndex(cpID, logIndex int64) error {
	_, err := s.db.Exec(
		`UPDATE checkpoints SET rekor_log_index=? WHERE id=?`, logIndex, cpID)
	return err
}

func (s *SQLiteStore) AddWitnessSignature(cpID int64, witnessID, sigHex string) error {
	_, err := s.db.Exec(
		`INSERT INTO witness_signatures (checkpoint_id, witness_id, sig_hex)
		 VALUES (?, ?, ?)
		 ON CONFLICT(checkpoint_id, witness_id) DO UPDATE SET sig_hex=excluded.sig_hex`,
		cpID, witnessID, sigHex)
	return err
}

func (s *SQLiteStore) WitnessSignatures(cpID int64) (map[string]string, error) {
	rows, err := s.db.Query(
		`SELECT witness_id, sig_hex FROM witness_signatures WHERE checkpoint_id=?`, cpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sigs := map[string]string{}
	for rows.Next() {
		var wid, sig string
		if err := rows.Scan(&wid, &sig); err != nil {
			return nil, err
		}
		sigs[wid] = sig
	}
	return sigs, rows.Err()
}

func (s *SQLiteStore) Stats() (*StoreStats, error) {
	var st StoreStats
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM receipts`).Scan(&st.TotalReceipts); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM receipts WHERE batched=0`).Scan(&st.PendingBatch); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM checkpoints`).Scan(&st.TotalCheckpoints); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM witness_signatures`).Scan(&st.TotalWitnessSigs); err != nil {
		return nil, err
	}
	return &st, nil
}

// --- helpers ---

func scanStoredReceipts(rows *sql.Rows) ([]StoredReceipt, error) {
	var out []StoredReceipt
	for rows.Next() {
		var rowID int64
		var js string
		if err := rows.Scan(&rowID, &js); err != nil {
			return nil, err
		}
		var r vur.Receipt
		if err := json.Unmarshal([]byte(js), &r); err != nil {
			return nil, fmt.Errorf("unmarshal receipt %d: %w", rowID, err)
		}
		out = append(out, StoredReceipt{RowID: rowID, Receipt: r})
	}
	return out, rows.Err()
}

func scanReceipts(rows *sql.Rows) ([]vur.Receipt, error) {
	var receipts []vur.Receipt
	for rows.Next() {
		var id int64
		var js string
		if err := rows.Scan(&id, &js); err != nil {
			return nil, err
		}
		var r vur.Receipt
		if err := json.Unmarshal([]byte(js), &r); err != nil {
			return nil, fmt.Errorf("unmarshal receipt %d: %w", id, err)
		}
		receipts = append(receipts, r)
	}
	return receipts, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCheckpoint(s scanner) (*Checkpoint, error) {
	var cp Checkpoint
	var issuedUnix int64
	err := s.Scan(
		&cp.ID, &cp.BatchStart, &cp.BatchEnd, &cp.TreeSize,
		&cp.RootHex, &cp.PreviousRootHex, &issuedUnix, &cp.OperatorSigHex, &cp.RekorLogIndex,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cp.IssuedAt = time.Unix(issuedUnix, 0).UTC()
	return &cp, nil
}

func scanCheckpointRow(rows *sql.Rows) (*Checkpoint, error) {
	var cp Checkpoint
	var issuedUnix int64
	err := rows.Scan(
		&cp.ID, &cp.BatchStart, &cp.BatchEnd, &cp.TreeSize,
		&cp.RootHex, &cp.PreviousRootHex, &issuedUnix, &cp.OperatorSigHex, &cp.RekorLogIndex,
	)
	if err != nil {
		return nil, err
	}
	cp.IssuedAt = time.Unix(issuedUnix, 0).UTC()
	return &cp, nil
}

// AppendToLog is not supported by the SQLite (legacy) backend.
func (s *SQLiteStore) AppendToLog(_ []byte) (uint64, error) {
	return 0, fmt.Errorf("AppendToLog: not supported by SQLite backend; use Tessera backend")
}

// GetLogInclusionProof is not supported by the SQLite (legacy) backend.
func (s *SQLiteStore) GetLogInclusionProof(_, _ uint64) (*InclusionProofResult, error) {
	return nil, fmt.Errorf("GetLogInclusionProof: not supported by SQLite backend; use Tessera backend")
}

// GetLogConsistencyProof is not supported by the SQLite (legacy) backend.
func (s *SQLiteStore) GetLogConsistencyProof(_, _ uint64) (*ConsistencyProofResult, error) {
	return nil, fmt.Errorf("GetLogConsistencyProof: not supported by SQLite backend; use Tessera backend")
}

// GetLogCheckpoint is not supported by the SQLite (legacy) backend.
func (s *SQLiteStore) GetLogCheckpoint() (*LogCheckpoint, error) {
	return nil, fmt.Errorf("GetLogCheckpoint: not supported by SQLite backend; use Tessera backend")
}
