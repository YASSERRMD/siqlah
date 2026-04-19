package test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// makeTesseraStore creates a TesseraStore backed by a temp directory.
// Returns the store and a cleanup function.
func makeTesseraStore(b *testing.B) (*store.TesseraStore, func()) {
	b.Helper()
	dir, err := os.MkdirTemp("", "tessera-bench-*")
	if err != nil {
		b.Fatalf("mktemp: %v", err)
	}
	dbPath := dir + "/bench.db"
	tessPath := dir + "/tlog/"

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatalf("keygen: %v", err)
	}

	ctx := context.Background()
	ts, err := store.NewTesseraStore(ctx, dbPath, tessPath, "bench.siqlah.dev/log", priv)
	if err != nil {
		os.RemoveAll(dir)
		b.Fatalf("open tessera store: %v", err)
	}
	return ts, func() {
		ts.Close()
		os.RemoveAll(dir)
	}
}

func BenchmarkTesseraAppendReceipt(b *testing.B) {
	ts, cleanup := makeTesseraStore(b)
	defer cleanup()

	r := sampleReceipt()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ID = fmt.Sprintf("bench-%d", i)
		if _, err := ts.AppendReceipt(*r); err != nil {
			b.Fatalf("append: %v", err)
		}
	}
}

func BenchmarkTesseraAppendReceipt_Parallel(b *testing.B) {
	ts, cleanup := makeTesseraStore(b)
	defer cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			r := &vur.Receipt{
				ID:           fmt.Sprintf("par-%d", i),
				Version:      "1.0.0",
				Provider:     "openai",
				Model:        "gpt-4o",
				InputTokens:  100,
				OutputTokens: 50,
			}
			if _, err := ts.AppendReceipt(*r); err != nil {
				b.Errorf("append: %v", err)
			}
			i++
		}
	})
}

func BenchmarkTesseraGetReceiptByID(b *testing.B) {
	ts, cleanup := makeTesseraStore(b)
	defer cleanup()

	// Pre-populate.
	ids := make([]string, 100)
	for i := range ids {
		id := fmt.Sprintf("preload-%d", i)
		ids[i] = id
		ts.AppendReceipt(vur.Receipt{ID: id, Provider: "openai", Model: "gpt-4o"})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		if _, err := ts.GetReceiptByID(id); err != nil {
			b.Fatalf("get: %v", err)
		}
	}
}

func BenchmarkTesseraFetchUnbatched(b *testing.B) {
	ts, cleanup := makeTesseraStore(b)
	defer cleanup()

	for i := 0; i < 500; i++ {
		ts.AppendReceipt(vur.Receipt{
			ID:           fmt.Sprintf("unbatched-%d", i),
			Provider:     "openai",
			Model:        "gpt-4o",
			InputTokens:  100,
			OutputTokens: 50,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ts.FetchUnbatched(100); err != nil {
			b.Fatalf("fetch: %v", err)
		}
	}
}
