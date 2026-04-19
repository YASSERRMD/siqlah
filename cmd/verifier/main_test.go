package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/merkle"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func newSignedReceipt(t *testing.T) (*vur.Receipt, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := &vur.Receipt{
		ID:             "test-receipt-id-001",
		Version:        "1.0.0",
		Tenant:         "test-tenant",
		Provider:       "openai",
		Model:          "gpt-4o",
		InputTokens:    100,
		OutputTokens:   50,
		Timestamp:      time.Now().UTC(),
		SignerIdentity: hex.EncodeToString(pub),
		Verified:       true,
	}
	if err := vur.SignReceipt(r, priv); err != nil {
		t.Fatalf("sign receipt: %v", err)
	}
	return r, pub
}

func writeReceiptFile(t *testing.T, r *vur.Receipt) string {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(f, b, 0644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestVerifyReceiptValid(t *testing.T) {
	r, pub := newSignedReceipt(t)
	f := writeReceiptFile(t, r)
	if err := runVerifyReceipt([]string{
		"--receipt", f,
		"--pub", hex.EncodeToString(pub),
	}); err != nil {
		t.Errorf("expected valid receipt, got: %v", err)
	}
}

func TestVerifyReceiptInvalidSig(t *testing.T) {
	r, _ := newSignedReceipt(t)
	// Use a different key to verify — should fail.
	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)
	f := writeReceiptFile(t, r)
	err := runVerifyReceipt([]string{
		"--receipt", f,
		"--pub", hex.EncodeToString(wrongPub),
	})
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestVerifyTokensWithVerifiedCounts(t *testing.T) {
	r, _ := newSignedReceipt(t)
	r.VerifiedInputTokens = 95
	r.VerifiedOutputTokens = 48
	f := writeReceiptFile(t, r)
	if err := runVerifyTokens([]string{"--receipt", f}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyTokensNoVerification(t *testing.T) {
	r, _ := newSignedReceipt(t)
	// No verified token counts.
	f := writeReceiptFile(t, r)
	if err := runVerifyTokens([]string{"--receipt", f}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckProofValid(t *testing.T) {
	r, _ := newSignedReceipt(t)

	// Build a real inclusion proof.
	cb, err := r.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	leaf := merkle.HashLeaf(cb)
	leaves := [][32]byte{leaf}
	root := merkle.BuildRoot(leaves)
	rootHex := merkle.FormatRoot(root)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"receipt_id":    "test-receipt-id-001",
			"checkpoint_id": 1,
			"leaf_index":    0,
			"tree_size":     1,
			"root_hex":      rootHex,
			"proof":         []string{},
		})
	}))
	defer srv.Close()

	f := writeReceiptFile(t, r)
	if err := runCheckProof([]string{
		"--receipt", f,
		"--ledger", srv.URL,
	}); err != nil {
		t.Errorf("expected valid proof: %v", err)
	}
}

func TestCheckProofInvalid(t *testing.T) {
	r, _ := newSignedReceipt(t)

	// Use wrong root hex.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"receipt_id":    "test-receipt-id-001",
			"checkpoint_id": 1,
			"leaf_index":    0,
			"tree_size":     1,
			"root_hex":      "0000000000000000000000000000000000000000000000000000000000000000",
			"proof":         []string{},
		})
	}))
	defer srv.Close()

	f := writeReceiptFile(t, r)
	err := runCheckProof([]string{
		"--receipt", f,
		"--ledger", srv.URL,
	})
	if err == nil {
		t.Error("expected error for invalid proof")
	}
}

func TestReconcileAllMatch(t *testing.T) {
	r1, _ := newSignedReceipt(t)
	r2, _ := newSignedReceipt(t)
	r2.ID = "test-receipt-id-002"

	localLog := filepath.Join(t.TempDir(), "log.json")
	data, _ := json.Marshal([]vur.Receipt{*r1, *r2})
	os.WriteFile(localLog, data, 0644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return matching receipt.
		if r.URL.Path == "/v1/receipts/test-receipt-id-001" {
			json.NewEncoder(w).Encode(r1)
		} else {
			json.NewEncoder(w).Encode(r2)
		}
	}))
	defer srv.Close()

	if err := runReconcile([]string{
		"--ledger", srv.URL,
		"--local-log", localLog,
	}); err != nil {
		t.Errorf("expected clean reconcile: %v", err)
	}
}

func TestReconcileDiscrepancyDetected(t *testing.T) {
	r, _ := newSignedReceipt(t)

	// Remote receipt has inflated token count.
	remoteR := *r
	remoteR.InputTokens = 200 // 100% more than local

	localLog := filepath.Join(t.TempDir(), "log.json")
	data, _ := json.Marshal([]vur.Receipt{*r})
	os.WriteFile(localLog, data, 0644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(remoteR)
	}))
	defer srv.Close()

	err := runReconcile([]string{
		"--ledger", srv.URL,
		"--local-log", localLog,
		"--threshold", "5",
	})
	if err == nil {
		t.Error("expected discrepancy error")
	}
}

func TestDiscrepancyCalc(t *testing.T) {
	tests := []struct {
		a, b int64
		want float64
	}{
		{100, 100, 0},
		{100, 90, 10},
		{100, 110, 10},
		{0, 0, 0},
		{100, 0, 100},
	}
	for _, tt := range tests {
		got := discrepancy(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("discrepancy(%d,%d)=%.2f, want %.2f", tt.a, tt.b, got, tt.want)
		}
	}
}
