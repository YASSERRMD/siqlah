package test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/yasserrmd/siqlah/internal/api"
	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/provider"
	"github.com/yasserrmd/siqlah/internal/store"
)

func TestConcurrentIngest(t *testing.T) {
	const (
		numGoroutines    = 10
		receiptsPerGoroutine = 100 // keep total to 1000 for speed
	)

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	builder := checkpoint.NewBuilder(st, priv, 10000)
	reg := provider.NewRegistry()
	srv := api.New(st, builder, pub, priv, reg, "stress-test")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var successCount int64
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < receiptsPerGoroutine; i++ {
				respBody, _ := json.Marshal(map[string]any{
					"id": fmt.Sprintf("req-g%d-i%d", goroutineID, i),
					"usage": map[string]any{
						"prompt_tokens":     100 + i,
						"completion_tokens": 50 + i,
					},
				})
				body, _ := json.Marshal(map[string]any{
					"provider":      "openai",
					"tenant":        fmt.Sprintf("tenant-%d", goroutineID),
					"model":         "gpt-4o",
					"response_body": json.RawMessage(respBody),
				})
				resp, err := http.Post(ts.URL+"/v1/receipts",
					"application/json", bytes.NewReader(body))
				if err != nil {
					t.Logf("g%d i%d: POST error: %v", goroutineID, i, err)
					continue
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusCreated {
					atomic.AddInt64(&successCount, 1)
				} else {
					t.Logf("g%d i%d: unexpected status %d", goroutineID, i, resp.StatusCode)
				}
			}
		}(g)
	}

	wg.Wait()

	total := int64(numGoroutines * receiptsPerGoroutine)
	if successCount != total {
		t.Errorf("expected %d successful ingests, got %d", total, successCount)
	}
	t.Logf("stress test: %d/%d receipts ingested successfully", successCount, total)

	// Verify all receipts are stored.
	statsResp, _ := http.Get(ts.URL + "/v1/stats")
	var stats map[string]any
	json.NewDecoder(statsResp.Body).Decode(&stats)
	statsResp.Body.Close()

	if int64(stats["total_receipts"].(float64)) != total {
		t.Errorf("store has %v receipts, expected %d", stats["total_receipts"], total)
	}

	// Build checkpoint covering all receipts.
	cpResp, _ := http.Post(ts.URL+"/v1/checkpoints/build", "application/json", nil)
	if cpResp.StatusCode != http.StatusCreated {
		t.Errorf("checkpoint build: expected 201, got %d", cpResp.StatusCode)
	}
	cpResp.Body.Close()

	// Verify checkpoint operator signature.
	vResp, _ := http.Get(ts.URL + "/v1/checkpoints/1/verify")
	var verify map[string]any
	json.NewDecoder(vResp.Body).Decode(&verify)
	vResp.Body.Close()
	if verify["operator_valid"] != true {
		t.Error("checkpoint operator signature invalid after stress test")
	}
	t.Logf("checkpoint verified after stress test (%d receipts)", int(total))
}
