package vur_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

func sampleReceipt() vur.Receipt {
	return vur.Receipt{
		ID:                   "rec-001",
		Version:              "1.0.0",
		Tenant:               "acme-corp",
		Provider:             "openai",
		Model:                "gpt-4o",
		ModelDigest:          "",
		TokenizerID:          "cl100k_base",
		TokenizerHash:        "abc123",
		InputTokens:          100,
		OutputTokens:         50,
		ReasoningTokens:      0,
		VerifiedInputTokens:  98,
		VerifiedOutputTokens: 50,
		RequestHash:          "deadbeef",
		ResponseHash:         "cafebabe",
		TokenBoundaryRoot:    "aabbccdd",
		RequestID:            "req-xyz",
		Timestamp:            time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		SignerIdentity:       "test-key",
		Verified:             true,
		DiscrepancyPct:       2.0,
	}
}

func TestCanonicalDeterminism(t *testing.T) {
	r := sampleReceipt()
	b1, err := r.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	b2, err := r.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Errorf("canonical bytes not deterministic:\n%s\n%s", b1, b2)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := sampleReceipt()
	if err := vur.SignReceipt(&r, priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if r.SignatureHex == "" {
		t.Fatal("expected non-empty signature")
	}
	if err := vur.VerifyReceipt(&r, pub); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestTamperDetection(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := sampleReceipt()
	if err := vur.SignReceipt(&r, priv); err != nil {
		t.Fatal(err)
	}
	r.InputTokens = 9999
	if err := vur.VerifyReceipt(&r, pub); err == nil {
		t.Fatal("expected verification failure after tampering")
	}
}

func TestZeroValueReceipt(t *testing.T) {
	var r vur.Receipt
	b, err := r.CanonicalBytes()
	if err != nil {
		t.Fatalf("canonical bytes for zero value: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("expected non-empty canonical bytes")
	}
}

func TestVerifyWithoutSignature(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := sampleReceipt()
	if err := vur.VerifyReceipt(&r, pub); err == nil {
		t.Fatal("expected error when verifying unsigned receipt")
	}
}
