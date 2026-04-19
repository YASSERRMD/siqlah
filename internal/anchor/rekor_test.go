package anchor

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/store"
)

// mockRekorEntry builds the Rekor-style response body for a given logIndex.
func mockRekorEntry(uuid string, logIndex int64, dataHex string) []byte {
	bodyPayload := map[string]any{
		"kind":       "hashedrekord",
		"apiVersion": "0.0.1",
		"spec": map[string]any{
			"data": map[string]any{
				"hash": map[string]string{
					"algorithm": "sha256",
					"value":     dataHex,
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(bodyPayload)
	entry := map[string]any{
		"logIndex": logIndex,
		"body":     base64.StdEncoding.EncodeToString(bodyBytes),
	}
	resp := map[string]any{uuid: entry}
	b, _ := json.Marshal(resp)
	return b
}

func newMockRekorServer(t *testing.T) *httptest.Server {
	t.Helper()
	var logIndex int64
	var entries []struct {
		uuid    string
		index   int64
		dataHex string
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/log/entries":
			var body struct {
				Spec struct {
					Data struct {
						Hash struct {
							Value string `json:"value"`
						} `json:"hash"`
					} `json:"data"`
				} `json:"spec"`
			}
			json.NewDecoder(r.Body).Decode(&body)

			idx := atomic.AddInt64(&logIndex, 1)
			uuid := fmt.Sprintf("uuid-%d", idx)
			entries = append(entries, struct {
				uuid    string
				index   int64
				dataHex string
			}{uuid, idx, body.Spec.Data.Hash.Value})

			w.WriteHeader(http.StatusCreated)
			w.Write(mockRekorEntry(uuid, idx, body.Spec.Data.Hash.Value))

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/log/entries":
			idx := int64(0)
			fmt.Sscanf(r.URL.Query().Get("logIndex"), "%d", &idx)
			for _, e := range entries {
				if e.index == idx {
					w.Write(mockRekorEntry(e.uuid, e.index, e.dataHex))
					return
				}
			}
			http.NotFound(w, r)

		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestAnchorAndVerify(t *testing.T) {
	srv := newMockRekorServer(t)
	defer srv.Close()

	ra, err := NewRekorAnchor(srv.URL)
	if err != nil {
		t.Fatalf("NewRekorAnchor: %v", err)
	}

	checkpoint := []byte("siqlah-checkpoint\ntree_size=42\nroot=abc123\n")
	result, err := ra.Anchor(checkpoint)
	if err != nil {
		t.Fatalf("Anchor: %v", err)
	}
	if result.LogIndex <= 0 {
		t.Errorf("expected positive log index, got %d", result.LogIndex)
	}
	if result.EntryUUID == "" {
		t.Error("expected non-empty entry UUID")
	}

	if err := ra.VerifyAnchor(checkpoint, result.LogIndex); err != nil {
		t.Errorf("VerifyAnchor: %v", err)
	}
}

func TestVerifyAnchor_WrongCheckpoint(t *testing.T) {
	srv := newMockRekorServer(t)
	defer srv.Close()

	ra, err := NewRekorAnchor(srv.URL)
	if err != nil {
		t.Fatalf("NewRekorAnchor: %v", err)
	}

	cp1 := []byte("checkpoint-1")
	result, err := ra.Anchor(cp1)
	if err != nil {
		t.Fatalf("Anchor: %v", err)
	}

	// Verify with different checkpoint bytes — should fail.
	cp2 := []byte("checkpoint-2-different")
	if err := ra.VerifyAnchor(cp2, result.LogIndex); err == nil {
		t.Error("expected verification failure for wrong checkpoint")
	}
}

func TestVerifyAnchor_MissingEntry(t *testing.T) {
	srv := newMockRekorServer(t)
	defer srv.Close()

	ra, err := NewRekorAnchor(srv.URL)
	if err != nil {
		t.Fatalf("NewRekorAnchor: %v", err)
	}

	// No entries submitted yet; log index 999 does not exist.
	if err := ra.VerifyAnchor([]byte("cp"), 999); err == nil {
		t.Error("expected error for missing log entry")
	}
}

func TestAnchorUnavailable(t *testing.T) {
	// Use a server URL that immediately closes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ra, err := NewRekorAnchor(srv.URL)
	if err != nil {
		t.Fatalf("NewRekorAnchor: %v", err)
	}
	if _, err := ra.Anchor([]byte("cp")); err == nil {
		t.Error("expected error when Rekor is unavailable")
	}
}

// --- scheduler tests ---

type mockAnchorStore struct {
	cp         *store.Checkpoint
	updatedIdx int64
}

func (m *mockAnchorStore) LatestCheckpoint() (*store.Checkpoint, error) { return m.cp, nil }
func (m *mockAnchorStore) UpdateCheckpointRekorIndex(cpID, logIndex int64) error {
	m.updatedIdx = logIndex
	m.cp.RekorLogIndex = logIndex
	return nil
}

func TestSchedulerAnchorOnce(t *testing.T) {
	srv := newMockRekorServer(t)
	defer srv.Close()

	ra, _ := NewRekorAnchor(srv.URL)

	cp := &store.Checkpoint{
		ID:            1,
		TreeSize:      10,
		RootHex:       hex.EncodeToString(make([]byte, 32)),
		BatchStart:    1,
		BatchEnd:      10,
		IssuedAt:      time.Now(),
		RekorLogIndex: -1,
	}
	ms := &mockAnchorStore{cp: cp}
	sched := NewAnchorScheduler(ra, ms, time.Hour)

	if err := sched.AnchorOnce(); err != nil {
		t.Fatalf("AnchorOnce: %v", err)
	}
	if ms.updatedIdx <= 0 {
		t.Errorf("expected positive rekor log index stored, got %d", ms.updatedIdx)
	}
}

func TestSchedulerSkipsAlreadyAnchored(t *testing.T) {
	srv := newMockRekorServer(t)
	defer srv.Close()

	ra, _ := NewRekorAnchor(srv.URL)

	cp := &store.Checkpoint{ID: 1, RekorLogIndex: 42}
	ms := &mockAnchorStore{cp: cp}
	sched := NewAnchorScheduler(ra, ms, time.Hour)

	if err := sched.AnchorOnce(); err != nil {
		t.Fatalf("AnchorOnce: %v", err)
	}
	if ms.updatedIdx != 0 {
		t.Errorf("expected no update for already-anchored checkpoint, updatedIdx=%d", ms.updatedIdx)
	}
}

func TestSchedulerNilCheckpoint(t *testing.T) {
	ra, _ := NewRekorAnchor("http://localhost:0")
	ms := &mockAnchorStore{cp: nil}
	sched := NewAnchorScheduler(ra, ms, time.Hour)
	// Should return nil (nothing to anchor).
	if err := sched.AnchorOnce(); err != nil {
		t.Errorf("expected nil error for nil checkpoint, got %v", err)
	}
}
