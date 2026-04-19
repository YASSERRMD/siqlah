package attestation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

func makeReceipt(opts ...func(*vur.Receipt)) *vur.Receipt {
	r := &vur.Receipt{
		ID:           "test-receipt-001",
		Version:      "1.0",
		Tenant:       "test-tenant",
		Provider:     "openai",
		Model:        "gpt-4o",
		ModelDigest:  "sha256:abc123",
		InputTokens:  100,
		OutputTokens: 50,
		RequestHash:  "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		ResponseHash: "11223344112233441122334411223344112233441122334411223344112233dd",
		Timestamp:    time.Now(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func TestNewReceiptStatement_Basic(t *testing.T) {
	r := makeReceipt()
	stmt, err := NewReceiptStatement(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt == nil {
		t.Fatal("expected non-nil statement")
	}

	b, err := stmt.Bytes()
	if err != nil {
		t.Fatalf("Bytes() error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got := raw["_type"]; got != StatementTypeURI {
		t.Errorf("_type = %q, want %q", got, StatementTypeURI)
	}
	if got := raw["predicateType"]; got != PredicateTypeURI {
		t.Errorf("predicateType = %q, want %q", got, PredicateTypeURI)
	}
}

func TestNewReceiptStatement_Subjects(t *testing.T) {
	r := makeReceipt()
	stmt, _ := NewReceiptStatement(r)
	b, _ := stmt.Bytes()

	var s Statement
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(s.Subject) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(s.Subject))
	}

	reqSub := s.Subject[0]
	if reqSub.Name != "request:"+r.ID {
		t.Errorf("subject[0].name = %q", reqSub.Name)
	}
	if reqSub.Digest["sha256"] != r.RequestHash {
		t.Errorf("subject[0].digest.sha256 = %q", reqSub.Digest["sha256"])
	}

	respSub := s.Subject[1]
	if respSub.Name != "response:"+r.ID {
		t.Errorf("subject[1].name = %q", respSub.Name)
	}
	if respSub.Digest["sha256"] != r.ResponseHash {
		t.Errorf("subject[1].digest.sha256 = %q", respSub.Digest["sha256"])
	}
}

func TestNewReceiptStatement_ModelProvenance(t *testing.T) {
	r := makeReceipt(func(r *vur.Receipt) {
		r.ModelSignerIdentity = "signer@example.com"
		r.ModelSignatureVerified = true
	})

	stmt, _ := NewReceiptStatement(r)
	b, _ := stmt.Bytes()

	var raw map[string]any
	json.Unmarshal(b, &raw)
	pred := raw["predicate"].(map[string]any)
	mp, ok := pred["model_provenance"].(map[string]any)
	if !ok {
		t.Fatal("expected model_provenance in predicate")
	}
	if mp["signer_identity"] != "signer@example.com" {
		t.Errorf("signer_identity = %v", mp["signer_identity"])
	}
	if mp["verified"] != true {
		t.Errorf("verified = %v", mp["verified"])
	}
}

func TestNewReceiptStatement_NoModelProvenance(t *testing.T) {
	r := makeReceipt() // no model identity fields
	stmt, _ := NewReceiptStatement(r)
	b, _ := stmt.Bytes()

	var raw map[string]any
	json.Unmarshal(b, &raw)
	pred := raw["predicate"].(map[string]any)
	if _, ok := pred["model_provenance"]; ok {
		t.Error("model_provenance should be absent when not set")
	}
}

func TestNewReceiptStatement_EnergyFootprint(t *testing.T) {
	r := makeReceipt(func(r *vur.Receipt) {
		r.EnergyEstimateJoules = 3600.0
		r.CarbonIntensityGCO2ePerKWh = 200.0
		r.InferenceRegion = "us-east-1"
	})

	stmt, _ := NewReceiptStatement(r)
	b, _ := stmt.Bytes()

	var raw map[string]any
	json.Unmarshal(b, &raw)
	pred := raw["predicate"].(map[string]any)
	ef, ok := pred["energy_footprint"].(map[string]any)
	if !ok {
		t.Fatal("expected energy_footprint in predicate")
	}
	if ef["joules"] != 3600.0 {
		t.Errorf("joules = %v", ef["joules"])
	}
	if ef["region"] != "us-east-1" {
		t.Errorf("region = %v", ef["region"])
	}
}

func TestNewReceiptStatement_NilReceipt(t *testing.T) {
	_, err := NewReceiptStatement(nil)
	if err == nil {
		t.Error("expected error for nil receipt")
	}
}

func TestNewReceiptStatement_MissingHashes(t *testing.T) {
	r := makeReceipt(func(r *vur.Receipt) {
		r.RequestHash = ""
		r.ResponseHash = ""
	})
	_, err := NewReceiptStatement(r)
	if err == nil {
		t.Error("expected error when request/response hashes are missing")
	}
}
