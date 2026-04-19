package test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// TestCompat_V1ReceiptDeserializes verifies that a v1-era JSON receipt
// (without the newer optional fields) still deserializes cleanly.
func TestCompat_V1ReceiptDeserializes(t *testing.T) {
	v1JSON := `{
		"id": "compat-v1-001",
		"version": "1.0.0",
		"tenant": "tenant-a",
		"provider": "openai",
		"model": "gpt-4",
		"model_digest": "",
		"tokenizer_id": "",
		"tokenizer_hash": "",
		"input_tokens": 100,
		"output_tokens": 50,
		"reasoning_tokens": 0,
		"verified_input_tokens": 0,
		"verified_output_tokens": 0,
		"request_hash": "abc123",
		"response_hash": "def456",
		"token_boundary_root": "",
		"request_id": "req-001",
		"timestamp": "2025-01-01T00:00:00Z",
		"signer_identity": "deadbeef",
		"signature_hex": "cafebabe",
		"verified": true,
		"discrepancy_pct": 0.0
	}`

	var r vur.Receipt
	if err := json.Unmarshal([]byte(v1JSON), &r); err != nil {
		t.Fatalf("unmarshal v1 receipt: %v", err)
	}
	if r.ID != "compat-v1-001" {
		t.Errorf("ID = %q, want compat-v1-001", r.ID)
	}
	if r.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", r.InputTokens)
	}
	// Optional Rekor fields must be zero-valued.
	if r.RekorLogIndex != 0 {
		t.Errorf("RekorLogIndex = %d, want 0 for v1 receipt", r.RekorLogIndex)
	}
	if r.CertificatePEM != "" {
		t.Errorf("CertificatePEM = %q, want empty for v1 receipt", r.CertificatePEM)
	}
}

// TestCompat_V11ReceiptRoundtrip verifies that v1.1 Fulcio/Rekor fields survive
// JSON marshal/unmarshal without loss.
func TestCompat_V11ReceiptRoundtrip(t *testing.T) {
	r := &vur.Receipt{
		ID:             "compat-v11-001",
		Version:        "1.1.0",
		Tenant:         "tenant-b",
		Provider:       "anthropic",
		Model:          "claude-3-5-sonnet",
		InputTokens:    1000,
		OutputTokens:   500,
		Timestamp:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		SignerType:     "fulcio",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\nMIIBIjAN...\n-----END CERTIFICATE-----",
		RekorLogIndex:  42,
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var r2 vur.Receipt
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r2.CertificatePEM != r.CertificatePEM {
		t.Errorf("CertificatePEM mismatch")
	}
	if r2.RekorLogIndex != r.RekorLogIndex {
		t.Errorf("RekorLogIndex mismatch: got %d, want %d", r2.RekorLogIndex, r.RekorLogIndex)
	}
	if r2.SignerType != "fulcio" {
		t.Errorf("SignerType = %q, want fulcio", r2.SignerType)
	}
}

// TestCompat_CanonicalBytesStable verifies that CanonicalBytes produces
// consistent output and that all expected fields appear in alphabetical order.
func TestCompat_CanonicalBytesStable(t *testing.T) {
	r := &vur.Receipt{
		ID:           "stable-001",
		Version:      "1.0.0",
		Tenant:       "t",
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  10,
		OutputTokens: 5,
		Timestamp:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	b1, err := r.CanonicalBytes()
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	b2, err := r.CanonicalBytes()
	if err != nil {
		t.Fatalf("CanonicalBytes (2nd): %v", err)
	}
	if string(b1) != string(b2) {
		t.Error("CanonicalBytes is not stable across calls")
	}

	// Verify alphabetical field order by checking that "id" appears before "model".
	s := string(b1)
	idPos := strings.Index(s, `"id"`)
	modelPos := strings.Index(s, `"model"`)
	if idPos < 0 || modelPos < 0 {
		t.Fatal("expected id and model fields in canonical JSON")
	}
	if idPos > modelPos {
		t.Error("canonical JSON: 'id' should appear before 'model' (alphabetical order)")
	}
}

// TestCompat_OptionalFieldsOmitted verifies that optional fields with zero values
// are omitted from JSON serialization (omitempty semantics).
func TestCompat_OptionalFieldsOmitted(t *testing.T) {
	r := &vur.Receipt{
		ID:        "omit-compat-001",
		Version:   "1.1.0",
		Provider:  "anthropic",
		Model:     "claude-opus-4",
		Timestamp: time.Now().UTC(),
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "certificate_pem") {
		t.Error("empty CertificatePEM should be omitted from JSON")
	}
	if strings.Contains(s, "rekor_log_index") {
		t.Error("zero RekorLogIndex should be omitted from JSON")
	}
}
