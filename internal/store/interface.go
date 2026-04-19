package store

import (
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// Checkpoint is a signed Merkle checkpoint over a batch of receipts.
type Checkpoint struct {
	ID              int64
	BatchStart      int64
	BatchEnd        int64
	TreeSize        int
	RootHex         string
	PreviousRootHex string
	IssuedAt        time.Time
	OperatorSigHex  string
}

// StoreStats holds aggregate statistics about the store.
type StoreStats struct {
	TotalReceipts     int64
	TotalCheckpoints  int64
	PendingBatch      int64
	TotalWitnessSigs  int64
}

// StoredReceipt pairs a DB row ID with the receipt it holds.
type StoredReceipt struct {
	RowID   int64
	Receipt vur.Receipt
}

// Store is the append-only storage interface for receipts and checkpoints.
type Store interface {
	// Receipt operations
	AppendReceipt(r vur.Receipt) (int64, error)
	GetReceiptByID(id string) (*StoredReceipt, error)
	FetchUnbatched(limit int) ([]StoredReceipt, error)
	MarkBatched(ids []int64) error
	GetReceiptsByRange(startID, endID int64) ([]vur.Receipt, error)

	// Checkpoint operations
	SaveCheckpoint(c Checkpoint) (int64, error)
	GetCheckpoint(id int64) (*Checkpoint, error)
	ListCheckpoints(offset, limit int) ([]Checkpoint, error)
	LatestCheckpoint() (*Checkpoint, error)

	// Witness operations
	AddWitnessSignature(cpID int64, witnessID, sigHex string) error
	WitnessSignatures(cpID int64) (map[string]string, error)

	// Utility
	Stats() (*StoreStats, error)
	Close() error
}
