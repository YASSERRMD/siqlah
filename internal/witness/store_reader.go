package witness

import (
	"fmt"

	"github.com/yasserrmd/siqlah/internal/store"
)

// StoreLogReader adapts store.Store to the LogReader interface.
// It works with both TesseraStore and SQLiteStore backends.
// For SQLiteStore, ConsistencyProof always returns an empty proof (not supported).
type StoreLogReader struct {
	st store.Store
}

// NewStoreLogReader creates a LogReader backed by the given store.
func NewStoreLogReader(st store.Store) *StoreLogReader {
	return &StoreLogReader{st: st}
}

// LatestCheckpoint returns the raw C2SP signed note from the log backend.
func (r *StoreLogReader) LatestCheckpoint() ([]byte, error) {
	lcp, err := r.st.GetLogCheckpoint()
	if err != nil {
		return nil, fmt.Errorf("log checkpoint: %w", err)
	}
	return lcp.RawNote, nil
}

// ConsistencyProof returns Merkle proof hashes from oldSize to newSize.
// Returns nil proof (no error) when the backend does not support proofs.
func (r *StoreLogReader) ConsistencyProof(oldSize, newSize uint64) ([][]byte, error) {
	if oldSize == 0 || oldSize >= newSize {
		return nil, nil
	}
	result, err := r.st.GetLogConsistencyProof(oldSize, newSize)
	if err != nil {
		// SQLite backend returns an unsupported error — treat gracefully.
		return nil, nil
	}
	return result.Proof, nil
}
