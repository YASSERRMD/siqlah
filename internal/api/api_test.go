package api_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yasserrmd/siqlah/internal/api"
	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/provider"
	"github.com/yasserrmd/siqlah/internal/store"
)

// openAIBody returns a minimal OpenAI response JSON for testing.
func openAIBody(inputTokens, outputTokens int) []byte {
	return []byte(`{"id":"req-test","usage":{"prompt_tokens":` +
		string(rune('0'+inputTokens)) + `,"completion_tokens":` +
		string(rune('0'+outputTokens)) + `}}`)
}

func openAIBodyFull(inputTokens, outputTokens int) []byte {
	b, _ := json.Marshal(map[string]any{
		"id": "req-test",
		"usage": map[string]any{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
		},
	})
	return b
}

func newTestServer(t *testing.T) (*api.Server, *store.SQLiteStore) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	builder := checkpoint.NewBuilder(st, priv, 1000)
	reg := provider.NewRegistry()
	srv := api.New(st, builder, pub, priv, reg, "test")
	return srv, st
}

func TestHealthEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
}

func TestIngestReceipt(t *testing.T) {
	srv, _ := newTestServer(t)

	body := map[string]any{
		"provider":      "openai",
		"tenant":        "test-tenant",
		"model":         "gpt-4o",
		"response_body": json.RawMessage(openAIBodyFull(100, 50)),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/receipts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var receipt map[string]any
	if err := json.NewDecoder(w.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["id"] == "" {
		t.Error("receipt id should not be empty")
	}
	if receipt["provider"] != "openai" {
		t.Errorf("unexpected provider: %v", receipt["provider"])
	}
	if receipt["input_tokens"].(float64) != 100 {
		t.Errorf("expected 100 input_tokens, got %v", receipt["input_tokens"])
	}
}

func TestIngestUnknownProvider(t *testing.T) {
	srv, _ := newTestServer(t)
	body := map[string]any{
		"provider":      "unknown-provider",
		"tenant":        "t",
		"model":         "m",
		"response_body": json.RawMessage(`{}`),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/receipts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetReceiptByID(t *testing.T) {
	srv, _ := newTestServer(t)

	// Ingest first.
	body := map[string]any{
		"provider":      "openai",
		"tenant":        "t",
		"model":         "gpt-4o",
		"response_body": json.RawMessage(openAIBodyFull(10, 5)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/receipts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("ingest failed: %d %s", w.Code, w.Body.String())
	}
	var ingested map[string]any
	json.NewDecoder(w.Body).Decode(&ingested)
	id := ingested["id"].(string)

	// Get by ID.
	req2 := httptest.NewRequest(http.MethodGet, "/v1/receipts/"+id, nil)
	w2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	var fetched map[string]any
	json.NewDecoder(w2.Body).Decode(&fetched)
	if fetched["id"] != id {
		t.Errorf("expected id %s, got %v", id, fetched["id"])
	}
}

func TestGetReceiptNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/receipts/nonexistent-uuid", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestBuildCheckpoint(t *testing.T) {
	srv, _ := newTestServer(t)

	// Ingest a receipt first.
	body := map[string]any{
		"provider":      "openai",
		"tenant":        "t",
		"model":         "gpt-4o",
		"response_body": json.RawMessage(openAIBodyFull(10, 5)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/receipts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("ingest failed: %d", w.Code)
	}

	// Build checkpoint.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/checkpoints/build", nil)
	w2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w2.Code, w2.Body.String())
	}
	var cp map[string]any
	json.NewDecoder(w2.Body).Decode(&cp)
	if cp["root_hex"] == "" {
		t.Error("expected non-empty root_hex")
	}
}

func TestFullFlow(t *testing.T) {
	srv, _ := newTestServer(t)

	// 1. Ingest a receipt.
	body := map[string]any{
		"provider":      "openai",
		"tenant":        "acme",
		"model":         "gpt-4o",
		"response_body": json.RawMessage(openAIBodyFull(200, 80)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/receipts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}
	var ingested map[string]any
	json.NewDecoder(w.Body).Decode(&ingested)
	receiptID := ingested["id"].(string)

	// 2. Build checkpoint.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/checkpoints/build", nil)
	w2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("build checkpoint: %d %s", w2.Code, w2.Body.String())
	}
	var cp map[string]any
	json.NewDecoder(w2.Body).Decode(&cp)
	if cp["RootHex"] == "" && cp["root_hex"] == "" {
		t.Error("checkpoint should have a root_hex")
	}

	// 3. Verify checkpoint.
	req3 := httptest.NewRequest(http.MethodGet, "/v1/checkpoints/1/verify", nil)
	w3 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", w3.Code, w3.Body.String())
	}
	var vresp map[string]any
	json.NewDecoder(w3.Body).Decode(&vresp)
	if vresp["operator_valid"] != true {
		t.Errorf("expected operator_valid=true, got %v", vresp["operator_valid"])
	}

	// 4. Submit witness signature (verification is not enforced at the HTTP layer).
	witnessReq := map[string]string{
		"witness_id": "witness-test-1",
		"sig_hex":    "aabbccdd",
	}
	wb, _ := json.Marshal(witnessReq)
	req4 := httptest.NewRequest(http.MethodPost, "/v1/checkpoints/1/witness", bytes.NewReader(wb))
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w4, req4)
	if w4.Code != http.StatusCreated {
		t.Fatalf("witness submit: %d %s", w4.Code, w4.Body.String())
	}

	// 5. Get inclusion proof.
	req5 := httptest.NewRequest(http.MethodGet, "/v1/receipts/"+receiptID+"/proof", nil)
	w5 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("inclusion proof: %d %s", w5.Code, w5.Body.String())
	}
	var proof map[string]any
	json.NewDecoder(w5.Body).Decode(&proof)
	if proof["root_hex"] == "" {
		t.Error("expected non-empty root_hex in proof")
	}
	if proof["leaf_index"].(float64) != 0 {
		t.Errorf("expected leaf_index=0, got %v", proof["leaf_index"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var stats map[string]any
	json.NewDecoder(w.Body).Decode(&stats)
	if _, ok := stats["total_receipts"]; !ok {
		t.Error("missing total_receipts")
	}
}

func TestIngestBatch(t *testing.T) {
	srv, _ := newTestServer(t)
	batchReq := map[string]any{
		"items": []map[string]any{
			{
				"provider":      "openai",
				"tenant":        "t",
				"model":         "gpt-4o",
				"response_body": json.RawMessage(openAIBodyFull(10, 5)),
			},
			{
				"provider":      "openai",
				"tenant":        "t",
				"model":         "gpt-4o",
				"response_body": json.RawMessage(openAIBodyFull(20, 10)),
			},
		},
	}
	b, _ := json.Marshal(batchReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/receipts/batch", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"].(float64) != 2 {
		t.Errorf("expected count=2, got %v", resp["count"])
	}
}

