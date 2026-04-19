package store_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func openTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleReceipt(n int) vur.Receipt {
	return vur.Receipt{
		ID:           fmt.Sprintf("rec-%03d", n),
		Version:      "1.0.0",
		Tenant:       "test",
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  int64(100 + n),
		OutputTokens: int64(50 + n),
		Timestamp:    time.Now().UTC(),
	}
}

func TestAppendFetch(t *testing.T) {
	s := openTestStore(t)
	r := sampleReceipt(1)
	id, err := s.AppendReceipt(r)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}
	stored, err := s.FetchUnbatched(10)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(stored))
	}
	if stored[0].Receipt.ID != r.ID {
		t.Errorf("receipt ID mismatch: got %s, want %s", stored[0].Receipt.ID, r.ID)
	}
}

func TestMarkBatched(t *testing.T) {
	s := openTestStore(t)
	var ids []int64
	for i := 0; i < 5; i++ {
		id, err := s.AppendReceipt(sampleReceipt(i))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := s.MarkBatched(ids[:3]); err != nil {
		t.Fatalf("mark batched: %v", err)
	}
	remaining, err := s.FetchUnbatched(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 unbatched, got %d", len(remaining))
	}
	_ = remaining
}

func TestCheckpointSaveList(t *testing.T) {
	s := openTestStore(t)
	cp := store.Checkpoint{
		BatchStart:      1,
		BatchEnd:        10,
		TreeSize:        10,
		RootHex:         "deadbeef",
		PreviousRootHex: "",
		IssuedAt:        time.Now().UTC().Truncate(time.Second),
		OperatorSigHex:  "cafebabe",
	}
	id, err := s.SaveCheckpoint(cp)
	if err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	got, err := s.GetCheckpoint(id)
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.RootHex != cp.RootHex {
		t.Errorf("root mismatch: got %s, want %s", got.RootHex, cp.RootHex)
	}
	list, err := s.ListCheckpoints(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(list))
	}
}

func TestWitnessSignatures(t *testing.T) {
	s := openTestStore(t)
	cpID, err := s.SaveCheckpoint(store.Checkpoint{
		BatchStart: 1, BatchEnd: 5, TreeSize: 5,
		RootHex: "aabb", IssuedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddWitnessSignature(cpID, "witness-1", "sig-aaa"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWitnessSignature(cpID, "witness-2", "sig-bbb"); err != nil {
		t.Fatal(err)
	}
	sigs, err := s.WitnessSignatures(cpID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 2 {
		t.Fatalf("expected 2 sigs, got %d", len(sigs))
	}
	if sigs["witness-1"] != "sig-aaa" {
		t.Errorf("wrong sig for witness-1")
	}
}

func TestConcurrentAppend(t *testing.T) {
	s := openTestStore(t)
	const goroutines = 10
	const perGoroutine = 100
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := s.AppendReceipt(sampleReceipt(g*perGoroutine + i)); err != nil {
					t.Errorf("goroutine %d append %d: %v", g, i, err)
				}
			}
		}(g)
	}
	wg.Wait()
	stats, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalReceipts != goroutines*perGoroutine {
		t.Errorf("expected %d receipts, got %d", goroutines*perGoroutine, stats.TotalReceipts)
	}
}
