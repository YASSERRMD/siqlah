package store

import (
	"encoding/hex"
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
	RekorLogIndex   int64 // -1 if not yet anchored to Rekor
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

// InclusionProofResult holds a Merkle inclusion proof from the Tessera-backed log.
type InclusionProofResult struct {
	LeafIndex uint64
	TreeSize  uint64
	RootHex   string
	Proof     [][]byte
}

// ConsistencyProofResult holds a Merkle consistency proof from the Tessera-backed log.
type ConsistencyProofResult struct {
	OldSize    uint64
	NewSize    uint64
	OldRootHex string
	NewRootHex string
	Proof      [][]byte
}

// LogCheckpoint holds the latest Tessera log checkpoint.
type LogCheckpoint struct {
	TreeSize uint64
	RootHex  string
	RawNote  []byte // raw C2SP signed note bytes
}

// ProofHexSlice converts [][]byte Merkle proof nodes to hex strings.
func ProofHexSlice(proof [][]byte) []string {
	out := make([]string, len(proof))
	for i, p := range proof {
		out[i] = hex.EncodeToString(p)
	}
	return out
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
	UpdateCheckpointRekorIndex(cpID, logIndex int64) error

	// Witness operations
	AddWitnessSignature(cpID int64, witnessID, sigHex string) error
	WitnessSignatures(cpID int64) (map[string]string, error)

	// Tessera-backed log operations (optional; return ErrNotSupported if not available)
	AppendToLog(receiptCanonicalBytes []byte) (logIndex uint64, err error)
	GetLogInclusionProof(receiptIndex, treeSize uint64) (*InclusionProofResult, error)
	GetLogConsistencyProof(oldSize, newSize uint64) (*ConsistencyProofResult, error)
	GetLogCheckpoint() (*LogCheckpoint, error)

	// Utility
	Stats() (*StoreStats, error)
	Close() error
}
