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
	witnesses []ExternalWitness
	client    *http.Client
	// lastSizes tracks the tree size most recently acknowledged by each witness.
	lastSizes map[string]uint64
}

// NewWitnessFeeder creates a WitnessFeeder targeting the provided external witnesses.
func NewWitnessFeeder(witnesses []ExternalWitness) *WitnessFeeder {
	return &WitnessFeeder{
		witnesses: witnesses,
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

// Feed submits rawCP (a C2SP signed note) to all configured external witnesses.
//
// consistencyProof should contain the Merkle consistency proof hashes from the
// last tree size the witness acknowledged to the current tree size. For a fresh
// witness (oldSize == 0) it must be nil or empty.
//
// Results are returned for every witness, regardless of individual errors.
func (f *WitnessFeeder) Feed(ctx context.Context, rawCP []byte, consistencyProof [][]byte) []FeedResult {
	results := make([]FeedResult, 0, len(f.witnesses))
	for _, w := range f.witnesses {
		result := f.feedOne(ctx, w, rawCP, consistencyProof)
		results = append(results, result)
	}
	return results
}

func (f *WitnessFeeder) feedOne(ctx context.Context, w ExternalWitness, rawCP []byte, proof [][]byte) FeedResult {
	u, err := url.Parse(w.URL)
	if err != nil {
		return FeedResult{WitnessName: w.Name, Err: fmt.Errorf("invalid witness URL: %w", err)}
	}

	old := f.lastSizes[w.Name]
	wc := witnesshttp.NewWitness(u, f.client)

	cosigned, newSize, err := wc.Update(ctx, old, rawCP, proof)
	if err != nil {
		// The witness returned a stale-checkpoint conflict — update our cached size
		// so the next call can supply the correct old size.
		if newSize > 0 {
			f.lastSizes[w.Name] = newSize
		}
		return FeedResult{WitnessName: w.Name, Err: fmt.Errorf("update witness %q: %w", w.Name, err)}
	}

	// Parse the returned cosigned note to extract the tree size for caching.
	if cp, err := ParseCheckpoint(string(cosigned)); err == nil && cp.TreeSize > 0 {
		f.lastSizes[w.Name] = cp.TreeSize
		log.Printf("witness feeder: %q cosigned tree_size=%d", w.Name, cp.TreeSize)
	}

	return FeedResult{WitnessName: w.Name, Cosigned: cosigned}
}
