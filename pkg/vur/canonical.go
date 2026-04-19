package vur

import (
	"encoding/json"
	"fmt"
)

// canonicalReceipt mirrors Receipt with explicit field ordering for deterministic JSON.
type canonicalReceipt struct {
	CertificatePEM         string  `json:"certificate_pem,omitempty"`
	DiscrepancyPct         float64 `json:"discrepancy_pct"`
	ID                     string  `json:"id"`
	InputTokens            int64   `json:"input_tokens"`
	Model                  string  `json:"model"`
	ModelDigest            string  `json:"model_digest"`
	ModelSignatureVerified bool    `json:"model_signature_verified,omitempty"`
	ModelSignerIdentity    string  `json:"model_signer_identity,omitempty"`
	OutputTokens           int64   `json:"output_tokens"`
	Provider               string  `json:"provider"`
	ReasoningTokens        int64   `json:"reasoning_tokens"`
	RekorLogIndex          int64   `json:"rekor_log_index,omitempty"`
	RequestHash            string  `json:"request_hash"`
	RequestID              string  `json:"request_id"`
	ResponseHash           string  `json:"response_hash"`
	SignerIdentity         string  `json:"signer_identity"`
	SignerType             string  `json:"signer_type"`
	Tenant                 string  `json:"tenant"`
	Timestamp              string  `json:"timestamp"`
	TokenBoundaryRoot      string  `json:"token_boundary_root"`
	TokenizerHash          string  `json:"tokenizer_hash"`
	TokenizerID            string  `json:"tokenizer_id"`
	Verified               bool    `json:"verified"`
	VerifiedInputTokens    int64   `json:"verified_input_tokens"`
	VerifiedOutputTokens   int64   `json:"verified_output_tokens"`
	Version                string  `json:"version"`
}

// CanonicalBytes returns deterministic JSON bytes suitable for signing.
// Fields are sorted alphabetically; timestamp is RFC3339Nano UTC; no whitespace.
func (r *Receipt) CanonicalBytes() ([]byte, error) {
	cr := canonicalReceipt{
		CertificatePEM:         r.CertificatePEM,
		DiscrepancyPct:         r.DiscrepancyPct,
		ID:                     r.ID,
		InputTokens:            r.InputTokens,
		Model:                  r.Model,
		ModelDigest:            r.ModelDigest,
		ModelSignatureVerified: r.ModelSignatureVerified,
		ModelSignerIdentity:    r.ModelSignerIdentity,
		OutputTokens:           r.OutputTokens,
		Provider:               r.Provider,
		ReasoningTokens:        r.ReasoningTokens,
		RekorLogIndex:          r.RekorLogIndex,
		RequestHash:            r.RequestHash,
		RequestID:              r.RequestID,
		ResponseHash:           r.ResponseHash,
		SignerIdentity:         r.SignerIdentity,
		SignerType:             r.SignerType,
		Tenant:                 r.Tenant,
		Timestamp:              r.Timestamp.UTC().Format("2006-01-02T15:04:05.999999999Z"),
		TokenBoundaryRoot:      r.TokenBoundaryRoot,
		TokenizerHash:          r.TokenizerHash,
		TokenizerID:            r.TokenizerID,
		Verified:               r.Verified,
		VerifiedInputTokens:    r.VerifiedInputTokens,
		VerifiedOutputTokens:   r.VerifiedOutputTokens,
		Version:                r.Version,
	}
	b, err := json.Marshal(cr)
	if err != nil {
		return nil, fmt.Errorf("canonical serialization: %w", err)
	}
	return b, nil
}
