package store

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"github.com/yasserrmd/siqlah/internal/tessera"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// TesseraStore composes the SQLite store with a Tessera append-only log.
// It satisfies the Store interface and routes Merkle-log operations to Tessera.
type TesseraStore struct {
	*SQLiteStore
	log *tessera.TesseraLog
	ctx context.Context
}

// NewTesseraStore creates a TesseraStore backed by SQLite at dbPath and
// Tessera POSIX storage at tesseraPath. privKey is used to sign Tessera checkpoints.
func NewTesseraStore(ctx context.Context, dbPath, tesseraPath, logName string, privKey ed25519.PrivateKey) (*TesseraStore, error) {
	sqlite, err := Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	signer, err := tessera.NewNoteSigner(logName, privKey)
	if err != nil {
		sqlite.Close()
		return nil, fmt.Errorf("create note signer: %w", err)
	}

	tlog, err := tessera.NewTesseraLog(ctx, tesseraPath, signer)
	if err != nil {
		sqlite.Close()
		return nil, fmt.Errorf("create tessera log: %w", err)
	}

	return &TesseraStore{SQLiteStore: sqlite, log: tlog, ctx: ctx}, nil
}

// AppendReceipt appends a receipt to SQLite and also appends its canonical bytes to the Tessera log.
// The log index is not stored in the receipt row yet (schema migration is additive-only).
func (ts *TesseraStore) AppendReceipt(r vur.Receipt) (int64, error) {
	rowID, err := ts.SQLiteStore.AppendReceipt(r)
	if err != nil {
		return 0, err
	}
	cb, err := r.CanonicalBytes()
	if err != nil {
		return rowID, fmt.Errorf("canonical bytes for log append: %w", err)
	}
	if _, err := ts.log.Append(ts.ctx, cb); err != nil {
		return rowID, fmt.Errorf("tessera append: %w", err)
	}
	return rowID, nil
}

// AppendToLog appends raw canonical bytes to the Tessera log and returns the log index.
func (ts *TesseraStore) AppendToLog(data []byte) (uint64, error) {
	return ts.log.Append(ts.ctx, data)
}

// GetLogInclusionProof returns a Merkle inclusion proof for the entry at receiptIndex
// within a tree of treeSize from the Tessera log.
func (ts *TesseraStore) GetLogInclusionProof(receiptIndex, treeSize uint64) (*InclusionProofResult, error) {
	proof, err := ts.log.InclusionProof(ts.ctx, receiptIndex, treeSize)
	if err != nil {
		return nil, err
	}
	root, err := ts.log.RootHash(ts.ctx)
	if err != nil {
		return nil, err
	}
	return &InclusionProofResult{
		LeafIndex: receiptIndex,
		TreeSize:  treeSize,
		RootHex:   hex.EncodeToString(root),
		Proof:     proof,
	}, nil
}

// GetLogConsistencyProof returns a Merkle consistency proof between oldSize and newSize.
func (ts *TesseraStore) GetLogConsistencyProof(oldSize, newSize uint64) (*ConsistencyProofResult, error) {
	proof, err := ts.log.ConsistencyProof(ts.ctx, oldSize, newSize)
	if err != nil {
		return nil, err
	}
	root, err := ts.log.RootHash(ts.ctx)
	if err != nil {
		return nil, err
	}
	// The current root is the new root; we don't store old roots in Tessera.
	return &ConsistencyProofResult{
		OldSize:    oldSize,
		NewSize:    newSize,
		NewRootHex: hex.EncodeToString(root),
		Proof:      proof,
	}, nil
}

// GetLogCheckpoint returns the latest Tessera log checkpoint in C2SP note format.
func (ts *TesseraStore) GetLogCheckpoint() (*LogCheckpoint, error) {
	raw, err := ts.log.ReadCheckpoint(ts.ctx)
	if err != nil {
		return nil, err
	}
	size, err := tessera.ParseCheckpointSize(raw)
	if err != nil {
		return nil, err
	}
	root, err := tessera.ParseCheckpointRoot(raw)
	if err != nil {
		return nil, err
	}
	return &LogCheckpoint{
		TreeSize: size,
		RootHex:  hex.EncodeToString(root),
		RawNote:  raw,
	}, nil
}

// Close shuts down both the Tessera log and the SQLite store.
func (ts *TesseraStore) Close() error {
	_ = ts.log.Close(ts.ctx)
	return ts.SQLiteStore.Close()
}
