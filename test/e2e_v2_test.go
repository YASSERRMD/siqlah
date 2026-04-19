package test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/api"
	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/provider"
	"github.com/yasserrmd/siqlah/internal/signing"
	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// newV2Server creates a v2 test server using NewWithOrigin constructor.
func newV2Server(t *testing.T) (*httptest.Server, *store.SQLiteStore, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	builder := checkpoint.NewBuilder(st, priv, 1000)
	reg := provider.NewRegistry()
	srv := api.NewWithOrigin(st, builder, pub, priv, reg, "v2-test", "test.siqlah.dev/log")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st, pub, priv
}

// TestV2_Ed25519SignerInterface verifies the Ed25519 Signer interface.
func TestV2_Ed25519SignerInterface(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := signing.NewEd25519Signer(priv)

	if signer.Type() != "ed25519" {
		t.Errorf("Type() = %q, want ed25519", signer.Type())
	}
	if signer.Identity() == "" {
		t.Error("Identity() is empty")
	}

	payload := []byte("test payload for signing")
	bundle, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if bundle.SignatureHex == "" {
		t.Error("SignatureHex is empty")
	}
	if bundle.SignerIdentity != signer.Identity() {
		t.Errorf("SignerIdentity mismatch")
	}
}

// TestV2_ReceiptSignVerify tests end-to-end receipt signing and verification.
func TestV2_ReceiptSignVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	r := &vur.Receipt{
		ID:             "e2e-v2-001",
		Version:        "1.2.0",
		Tenant:         "tenant-v2",
		Provider:       "openai",
		Model:          "gpt-4o",
		InputTokens:    512,
		OutputTokens:   256,
		RequestHash:    "deadbeef",
		ResponseHash:   "cafebabe",
		Timestamp:      time.Now().UTC(),
		SignerIdentity: fmt.Sprintf("%x", pub),
	}

	if err := vur.SignReceipt(r, priv); err != nil {
		t.Fatalf("SignReceipt: %v", err)
	}
	if r.SignatureHex == "" {
		t.Fatal("signature is empty after signing")
	}
	if err := vur.VerifyReceipt(r, pub); err != nil {
		t.Fatalf("VerifyReceipt: %v", err)
	}
}

// TestV2_HealthAndVersion checks the health endpoint returns the server version.
func TestV2_HealthAndVersion(t *testing.T) {
	ts, _, _, _ := newV2Server(t)

	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
	if body.Version != "v2-test" {
		t.Errorf("version = %q, want v2-test", body.Version)
	}
}

// TestV2_IngestAndCheckpoint tests the full ingest→checkpoint pipeline.
func TestV2_IngestAndCheckpoint(t *testing.T) {
	ts, _, _, _ := newV2Server(t)

	// Ingest several receipts.
	for i := 0; i < 5; i++ {
		id := ingestReceipt(t, ts.URL, "openai", "tenant-v2", "gpt-4o", 100+i, 50+i)
		if id == "" {
			t.Fatal("ingestReceipt returned empty ID")
		}
	}

	// Build a checkpoint.
	resp, err := http.Post(ts.URL+"/v1/checkpoints/build", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /v1/checkpoints/build: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	var cp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&cp); err != nil {
		t.Fatalf("decode checkpoint: %v", err)
	}
	if cp["TreeSize"] == nil {
		t.Error("TreeSize missing from checkpoint response")
	}
}

// TestV2_BatchIngest tests POST /v1/receipts/batch.
func TestV2_BatchIngest(t *testing.T) {
	ts, _, _, _ := newV2Server(t)

	makeItem := func(i int) map[string]any {
		respBody, _ := json.Marshal(map[string]any{
			"id": fmt.Sprintf("batch-req-%d", i),
			"usage": map[string]any{
				"prompt_tokens":     100 + i,
				"completion_tokens": 50 + i,
			},
		})
		return map[string]any{
			"provider":      "openai",
			"tenant":        "batch-tenant",
			"model":         "gpt-4o",
			"response_body": json.RawMessage(respBody),
		}
	}

	body, _ := json.Marshal(map[string]any{
		"items": []any{makeItem(1), makeItem(2), makeItem(3)},
	})

	resp, err := http.Post(ts.URL+"/v1/receipts/batch", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST batch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if count, ok := result["count"].(float64); !ok || int(count) != 3 {
		t.Errorf("count = %v, want 3", result["count"])
	}
}

// TestV2_WitnessSubmit tests the legacy witness submission endpoint.
func TestV2_WitnessSubmit(t *testing.T) {
	ts, _, pub, priv := newV2Server(t)

	// Ingest a receipt and build a checkpoint.
	ingestReceipt(t, ts.URL, "openai", "tenant", "gpt-4o", 100, 50)

	buildResp, err := http.Post(ts.URL+"/v1/checkpoints/build", "application/json", nil)
	if err != nil {
		t.Fatalf("POST checkpoints/build: %v", err)
	}
	if buildResp.StatusCode != http.StatusCreated {
		t.Fatalf("build checkpoint status = %d, want 201", buildResp.StatusCode)
	}
	var cp struct {
		ID int64
	}
	if err := json.NewDecoder(buildResp.Body).Decode(&cp); err != nil {
		t.Fatalf("decode checkpoint: %v", err)
	}
	buildResp.Body.Close()

	if cp.ID == 0 {
		t.Fatal("checkpoint ID is zero")
	}

	// Submit a witness signature.
	payload := fmt.Sprintf("checkpoint-%d", cp.ID)
	sig := ed25519.Sign(priv, []byte(payload))
	body, _ := json.Marshal(map[string]any{
		"witness_id": fmt.Sprintf("%x", pub),
		"sig_hex":    fmt.Sprintf("%x", sig),
	})
	resp, err := http.Post(
		fmt.Sprintf("%s/v1/checkpoints/%d/witness", ts.URL, cp.ID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST witness: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 or 201", resp.StatusCode)
	}
}
