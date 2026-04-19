package checkpoint

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/yasserrmd/siqlah/internal/merkle"
	"github.com/yasserrmd/siqlah/internal/store"
)

// Builder builds and signs Merkle checkpoints over batches of receipts.
type Builder struct {
	store      store.Store
	privateKey ed25519.PrivateKey
	maxBatch   int
}

// NewBuilder creates a Builder with the given store, signing key, and max batch size.
func NewBuilder(s store.Store, key ed25519.PrivateKey, maxBatch int) *Builder {
	if maxBatch <= 0 {
		maxBatch = 1000
	}
	return &Builder{store: s, privateKey: key, maxBatch: maxBatch}
}

// BuildAndSign fetches unbatched receipts, builds a Merkle checkpoint, signs it,
// saves it, and marks all receipts as batched.
// Returns nil, nil when there are no unbatched receipts.
func (b *Builder) BuildAndSign() (*store.Checkpoint, error) {
	stored, err := b.store.FetchUnbatched(b.maxBatch)
	if err != nil {
		return nil, fmt.Errorf("fetch unbatched: %w", err)
	}
	if len(stored) == 0 {
		return nil, nil
	}

	// Build Merkle root over canonical receipt bytes.
	leaves := make([][32]byte, len(stored))
	for i, sr := range stored {
		cb, err := sr.Receipt.CanonicalBytes()
		if err != nil {
			return nil, fmt.Errorf("canonical bytes for receipt %s: %w", sr.Receipt.ID, err)
		}
		leaves[i] = merkle.HashLeaf(cb)
	}
	root := merkle.BuildRoot(leaves)
	rootHex := merkle.FormatRoot(root)

	// Collect row IDs and batch bounds.
	rowIDs := make([]int64, len(stored))
	for i, sr := range stored {
		rowIDs[i] = sr.RowID
	}
	batchStart := rowIDs[0]
	batchEnd := rowIDs[len(rowIDs)-1]

	// Determine previous root from the last checkpoint.
	prevRootHex := ""
	if prev, _ := b.store.LatestCheckpoint(); prev != nil {
		prevRootHex = prev.RootHex
	}

	now := time.Now().UTC()
	payload := &SignedPayload{
		BatchStart:      batchStart,
		BatchEnd:        batchEnd,
		TreeSize:        len(leaves),
		RootHex:         rootHex,
		PreviousRootHex: prevRootHex,
		IssuedAt:        now.Format("2006-01-02T15:04:05.999999999Z"),
	}
	pb, err := payload.Bytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(b.privateKey, pb)

	cp := store.Checkpoint{
		BatchStart:      batchStart,
		BatchEnd:        batchEnd,
		TreeSize:        len(leaves),
		RootHex:         rootHex,
		PreviousRootHex: prevRootHex,
		IssuedAt:        now,
		OperatorSigHex:  hex.EncodeToString(sig),
	}
	id, err := b.store.SaveCheckpoint(cp)
	if err != nil {
		return nil, fmt.Errorf("save checkpoint: %w", err)
	}
	cp.ID = id

	if err := b.store.MarkBatched(rowIDs); err != nil {
		return nil, fmt.Errorf("mark batched: %w", err)
	}
	return &cp, nil
}
