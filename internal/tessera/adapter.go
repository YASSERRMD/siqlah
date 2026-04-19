package tessera

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/transparency-dev/tessera"
	"github.com/transparency-dev/tessera/client"
	"github.com/transparency-dev/tessera/storage/posix"
	"golang.org/x/mod/sumdb/note"
)

// TesseraLog wraps a Tessera POSIX-backed append-only log.
type TesseraLog struct {
	appender *tessera.Appender
	reader   tessera.LogReader
	shutdown func(context.Context) error
	ctx      context.Context
}

// NewTesseraLog creates a new POSIX-backed Tessera log at storagePath.
// signer is the note.Signer used to sign checkpoints.
func NewTesseraLog(ctx context.Context, storagePath string, signer note.Signer) (*TesseraLog, error) {
	if err := os.MkdirAll(storagePath, 0o755); err != nil {
		return nil, fmt.Errorf("create tessera storage dir: %w", err)
	}

	driver, err := posix.New(ctx, posix.Config{Path: storagePath})
	if err != nil {
		return nil, fmt.Errorf("create posix driver: %w", err)
	}

	opts := tessera.NewAppendOptions().
		WithCheckpointSigner(signer).
		WithBatching(256, 250*time.Millisecond).
		WithCheckpointInterval(5 * time.Second)

	appender, shutdown, reader, err := tessera.NewAppender(ctx, driver, opts)
	if err != nil {
		return nil, fmt.Errorf("create tessera appender: %w", err)
	}

	return &TesseraLog{
		appender: appender,
		reader:   reader,
		shutdown: shutdown,
		ctx:      ctx,
	}, nil
}

// Append adds raw bytes to the log. Returns the assigned log index.
func (t *TesseraLog) Append(ctx context.Context, data []byte) (uint64, error) {
	result, err := t.appender.Add(ctx, tessera.NewEntry(data))()
	if err != nil {
		return 0, fmt.Errorf("tessera append: %w", err)
	}
	return result.Index, nil
}

// TreeSize returns the current integrated (committed) tree size.
func (t *TesseraLog) TreeSize(ctx context.Context) (uint64, error) {
	cp, err := t.reader.ReadCheckpoint(ctx)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read checkpoint: %w", err)
	}
	// Parse the raw checkpoint note to extract tree size.
	parsed, err := ParseCheckpointSize(cp)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

// RootHash returns the Merkle root from the latest checkpoint.
func (t *TesseraLog) RootHash(ctx context.Context) ([]byte, error) {
	cp, err := t.reader.ReadCheckpoint(ctx)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	return ParseCheckpointRoot(cp)
}

// ReadCheckpoint returns the raw signed checkpoint bytes (C2SP note format).
func (t *TesseraLog) ReadCheckpoint(ctx context.Context) ([]byte, error) {
	return t.reader.ReadCheckpoint(ctx)
}

// InclusionProof returns the Merkle inclusion proof for the entry at index within a tree of treeSize.
func (t *TesseraLog) InclusionProof(ctx context.Context, index, treeSize uint64) ([][]byte, error) {
	pb, err := client.NewProofBuilder(ctx, treeSize, t.reader.ReadTile)
	if err != nil {
		return nil, fmt.Errorf("build proof builder: %w", err)
	}
	proof, err := pb.InclusionProof(ctx, index)
	if err != nil {
		return nil, fmt.Errorf("inclusion proof: %w", err)
	}
	return proof, nil
}

// ConsistencyProof returns the Merkle consistency proof between oldSize and newSize.
func (t *TesseraLog) ConsistencyProof(ctx context.Context, oldSize, newSize uint64) ([][]byte, error) {
	pb, err := client.NewProofBuilder(ctx, newSize, t.reader.ReadTile)
	if err != nil {
		return nil, fmt.Errorf("build proof builder: %w", err)
	}
	proof, err := pb.ConsistencyProof(ctx, oldSize, newSize)
	if err != nil {
		return nil, fmt.Errorf("consistency proof: %w", err)
	}
	return proof, nil
}

// ReadEntry returns the raw data bytes stored at the given log index.
func (t *TesseraLog) ReadEntry(ctx context.Context, index uint64) ([]byte, error) {
	treeSize, err := t.TreeSize(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := client.GetEntryBundle(ctx, t.reader.ReadEntryBundle, index/256, treeSize)
	if err != nil {
		return nil, fmt.Errorf("get entry bundle: %w", err)
	}
	offset := int(index % 256)
	if offset >= len(bundle.Entries) {
		return nil, fmt.Errorf("entry %d not found in bundle (bundle has %d entries)", index, len(bundle.Entries))
	}
	return bundle.Entries[offset], nil
}

// Close shuts down the Tessera appender cleanly.
func (t *TesseraLog) Close(ctx context.Context) error {
	return t.shutdown(ctx)
}
