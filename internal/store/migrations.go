package store

import "database/sql"

// Migrate applies all schema migrations in order.
func Migrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS receipts (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			receipt_json    TEXT    NOT NULL,
			batched         INTEGER NOT NULL DEFAULT 0,
			created_at      INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_receipts_batched ON receipts(batched, id)`,
		`CREATE TABLE IF NOT EXISTS checkpoints (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_start      INTEGER NOT NULL,
			batch_end        INTEGER NOT NULL,
			tree_size        INTEGER NOT NULL,
			root_hex         TEXT    NOT NULL,
			previous_root_hex TEXT   NOT NULL DEFAULT '',
			issued_at        INTEGER NOT NULL,
			operator_sig_hex TEXT    NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_checkpoints_id ON checkpoints(id)`,
		`CREATE TABLE IF NOT EXISTS witness_signatures (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			checkpoint_id INTEGER NOT NULL REFERENCES checkpoints(id),
			witness_id   TEXT    NOT NULL,
			sig_hex      TEXT    NOT NULL,
			UNIQUE(checkpoint_id, witness_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_witness_cp ON witness_signatures(checkpoint_id)`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return err
		}
	}
	return nil
}
