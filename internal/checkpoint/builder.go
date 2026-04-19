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
// It uses the legacy SQLite Merkle implementation. For Tessera-backed deployments
// use TesseraBuilder instead.
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

// LegacyBuilder is an alias for Builder retained for backward compatibility.
// Use NewBuilder for the SQLite-backed mode.
type LegacyBuilder = Builder

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

	// Truncate to whole seconds so the timestamp survives the SQLite Unix-second round-trip
	// and can be reconstructed identically for signature verification.
	now := time.Now().UTC().Truncate(time.Second)
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

// TesseraBuilder builds checkpoints by reading the current Tessera log state.
// It persists a record to the SQLite checkpoints table for backward compatibility.
type TesseraBuilder struct {
	store      store.Store
	privateKey ed25519.PrivateKey
	maxBatch   int
}

// NewTesseraBuilder creates a TesseraBuilder. The store must be a TesseraStore.
func NewTesseraBuilder(s store.Store, key ed25519.PrivateKey, maxBatch int) *TesseraBuilder {
	if maxBatch <= 0 {
		maxBatch = 1000
	}
	return &TesseraBuilder{store: s, privateKey: key, maxBatch: maxBatch}
}

// BuildAndSign reads the current Tessera checkpoint, signs it with the operator key,
// and persists a record to the SQLite checkpoints table.
// Returns nil, nil if the Tessera log is empty.
func (tb *TesseraBuilder) BuildAndSign() (*store.Checkpoint, error) {
	lcp, err := tb.store.GetLogCheckpoint()
	if err != nil {
		return nil, fmt.Errorf("get tessera checkpoint: %w", err)
	}
	if lcp == nil || lcp.TreeSize == 0 {
		return nil, nil
	}

	prevRootHex := ""
	if prev, _ := tb.store.LatestCheckpoint(); prev != nil {
		prevRootHex = prev.RootHex
	}

	now := time.Now().UTC().Truncate(time.Second)
	payload := &SignedPayload{
		BatchStart:      0,
		BatchEnd:        int64(lcp.TreeSize) - 1,
		TreeSize:        int(lcp.TreeSize),
		RootHex:         lcp.RootHex,
		PreviousRootHex: prevRootHex,
		IssuedAt:        now.Format("2006-01-02T15:04:05.999999999Z"),
	}
	pb, err := payload.Bytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(tb.privateKey, pb)

	cp := store.Checkpoint{
		BatchStart:      0,
		BatchEnd:        int64(lcp.TreeSize) - 1,
		TreeSize:        int(lcp.TreeSize),
		RootHex:         lcp.RootHex,
		PreviousRootHex: prevRootHex,
		IssuedAt:        now,
		OperatorSigHex:  hex.EncodeToString(sig),
	}
	id, err := tb.store.SaveCheckpoint(cp)
	if err != nil {
		return nil, fmt.Errorf("save checkpoint: %w", err)
	}
	cp.ID = id
	return &cp, nil
}
