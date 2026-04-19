package anchor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yasserrmd/siqlah/internal/store"
)

const DefaultAnchorInterval = 24 * time.Hour

// AnchorScheduler periodically anchors the latest siqlah checkpoint to Rekor.
type AnchorScheduler struct {
	anchor   *RekorAnchor
	store    anchorStore
	interval time.Duration
}

// anchorStore is the subset of store.Store used by the scheduler.
type anchorStore interface {
	LatestCheckpoint() (*store.Checkpoint, error)
	UpdateCheckpointRekorIndex(cpID, logIndex int64) error
}

// NewAnchorScheduler creates an AnchorScheduler.
func NewAnchorScheduler(a *RekorAnchor, st anchorStore, interval time.Duration) *AnchorScheduler {
	if interval <= 0 {
		interval = DefaultAnchorInterval
	}
	return &AnchorScheduler{anchor: a, store: st, interval: interval}
}

// Run starts the periodic anchoring loop; it blocks until ctx is cancelled.
func (s *AnchorScheduler) Run(ctx context.Context) {
	log.Printf("anchor: scheduler started, interval=%s", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.AnchorOnce(); err != nil {
				log.Printf("anchor: %v", err)
			}
		}
	}
}

// AnchorOnce performs a single anchoring attempt for the latest checkpoint.
func (s *AnchorScheduler) AnchorOnce() error {
	cp, err := s.store.LatestCheckpoint()
	if err != nil {
		return fmt.Errorf("fetch latest checkpoint: %w", err)
	}
	if cp == nil {
		return nil // nothing to anchor yet
	}
	if cp.RekorLogIndex >= 0 {
		return nil // already anchored
	}

	payload := checkpointPayload(*cp)
	result, err := s.anchor.Anchor(payload)
	if err != nil {
		return fmt.Errorf("anchor checkpoint %d: %w", cp.ID, err)
	}

	if err := s.store.UpdateCheckpointRekorIndex(cp.ID, result.LogIndex); err != nil {
		return fmt.Errorf("save rekor index for checkpoint %d: %w", cp.ID, err)
	}
	log.Printf("anchor: checkpoint %d anchored at Rekor log index %d (%s)", cp.ID, result.LogIndex, result.EntryURL)
	return nil
}

// checkpointPayload serialises the key checkpoint fields for signing.
func checkpointPayload(cp store.Checkpoint) []byte {
	return []byte(fmt.Sprintf("siqlah-checkpoint\ntree_size=%d\nroot=%s\nbatch=%d-%d\nissued_at=%s\n",
		cp.TreeSize, cp.RootHex, cp.BatchStart, cp.BatchEnd,
		cp.IssuedAt.UTC().Format("2006-01-02T15:04:05Z")))
}
