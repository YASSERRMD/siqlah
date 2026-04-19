package witness

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	witnesshttp "github.com/transparency-dev/witness/client/http"
	"golang.org/x/mod/sumdb/note"
)

// LogReader abstracts the source of checkpoints and consistency proofs.
// Both the Tessera and SQLite backends satisfy this interface.
type LogReader interface {
	// LatestCheckpoint returns the current log checkpoint as a raw C2SP signed note.
	LatestCheckpoint() ([]byte, error)
	// ConsistencyProof returns Merkle proof hashes from oldSize to newSize.
	// Returns nil proof (no error) when oldSize == 0.
	ConsistencyProof(oldSize, newSize uint64) ([][]byte, error)
}

// ExternalWitness holds connection details for a single external tlog-witness endpoint.
type ExternalWitness struct {
	// Name is a human-readable label (e.g. "sigsum-witness-1").
	Name string
	// URL is the base URL of the witness HTTP server (e.g. "https://witness.example.com").
	URL string
	// Verifier is the note.Verifier for the witness's public key.
	Verifier note.Verifier
}

// WitnessFeeder pushes siqlah checkpoints to one or more external tlog-witness
// endpoints using the C2SP tlog-witness protocol.
type WitnessFeeder struct {
	log       LogReader
	witnesses []ExternalWitness
	interval  time.Duration
	client    *http.Client
	// lastSizes tracks the tree size most recently acknowledged by each witness.
	lastSizes map[string]uint64
}

// NewWitnessFeeder creates a WitnessFeeder.
//
// logReader provides checkpoints and consistency proofs from the local log.
// When interval > 0, Run() polls on that interval; pass 0 for one-shot use via Feed().
func NewWitnessFeeder(logReader LogReader, witnesses []ExternalWitness, interval time.Duration) *WitnessFeeder {
	return &WitnessFeeder{
		log:       logReader,
		witnesses: witnesses,
		interval:  interval,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		lastSizes: make(map[string]uint64),
	}
}

// FeedResult holds the outcome of submitting a checkpoint to a single witness.
type FeedResult struct {
	WitnessName string
	// Cosigned contains the cosigned note bytes returned by the witness (nil on error).
	Cosigned []byte
	Err      error
}

// Run periodically fetches the latest checkpoint from the local log and pushes it
// to all configured external witnesses. It runs until ctx is cancelled.
func (f *WitnessFeeder) Run(ctx context.Context) {
	if f.interval <= 0 {
		f.interval = 60 * time.Second
	}
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	log.Printf("witness feeder: started, pushing to %d witness(es) every %s", len(f.witnesses), f.interval)
	f.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("witness feeder: stopped")
			return
		case <-ticker.C:
			f.runOnce(ctx)
		}
	}
}

func (f *WitnessFeeder) runOnce(ctx context.Context) {
	rawCP, err := f.log.LatestCheckpoint()
	if err != nil {
		log.Printf("witness feeder: fetch checkpoint: %v", err)
		return
	}

	cp, err := ParseCheckpoint(string(rawCP))
	if err != nil {
		log.Printf("witness feeder: parse checkpoint: %v", err)
		return
	}

	results := f.feedWithCheckpoint(ctx, rawCP, cp.TreeSize)
	for _, r := range results {
		if r.Err != nil {
			log.Printf("witness feeder: %s: %v", r.WitnessName, r.Err)
		}
	}
}

// Feed submits rawCP (a C2SP signed note) to all configured external witnesses.
// The proof is fetched automatically from the LogReader based on each witness's last known size.
// For one-shot use without a periodic Run().
func (f *WitnessFeeder) Feed(ctx context.Context, rawCP []byte, _ [][]byte) []FeedResult {
	cp, err := ParseCheckpoint(string(rawCP))
	if err != nil {
		results := make([]FeedResult, len(f.witnesses))
		for i, w := range f.witnesses {
			results[i] = FeedResult{WitnessName: w.Name, Err: fmt.Errorf("parse checkpoint: %w", err)}
		}
		return results
	}
	return f.feedWithCheckpoint(ctx, rawCP, cp.TreeSize)
}

func (f *WitnessFeeder) feedWithCheckpoint(ctx context.Context, rawCP []byte, newSize uint64) []FeedResult {
	results := make([]FeedResult, 0, len(f.witnesses))
	for _, w := range f.witnesses {
		result := f.feedOne(ctx, w, rawCP, newSize)
		results = append(results, result)
	}
	return results
}

func (f *WitnessFeeder) feedOne(ctx context.Context, w ExternalWitness, rawCP []byte, newSize uint64) FeedResult {
	u, err := url.Parse(w.URL)
	if err != nil {
		return FeedResult{WitnessName: w.Name, Err: fmt.Errorf("invalid witness URL: %w", err)}
	}

	old := f.lastSizes[w.Name]

	var proof [][]byte
	if old > 0 && f.log != nil {
		proof, err = f.log.ConsistencyProof(old, newSize)
		if err != nil {
			log.Printf("witness feeder: consistency proof %d→%d for %q: %v (sending without proof)", old, newSize, w.Name, err)
			proof = nil
			old = 0
		}
	}

	wc := witnesshttp.NewWitness(u, f.client)
	cosigned, newWitnessSize, err := wc.Update(ctx, old, rawCP, proof)
	if err != nil {
		if newWitnessSize > 0 {
			f.lastSizes[w.Name] = newWitnessSize
		}
		return FeedResult{WitnessName: w.Name, Err: fmt.Errorf("update witness %q: %w", w.Name, err)}
	}

	if cp, err := ParseCheckpoint(string(cosigned)); err == nil && cp.TreeSize > 0 {
		f.lastSizes[w.Name] = cp.TreeSize
		log.Printf("witness feeder: %q cosigned tree_size=%d", w.Name, cp.TreeSize)
	}

	return FeedResult{WitnessName: w.Name, Cosigned: cosigned}
}
