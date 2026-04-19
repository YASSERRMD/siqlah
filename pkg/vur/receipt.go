package vur

import "time"

// Receipt is a Verifiable Usage Receipt for a single AI API call.
type Receipt struct {
	ID                  string    `json:"id"`
	Version             string    `json:"version"`
	Tenant              string    `json:"tenant"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	ModelDigest         string    `json:"model_digest"`
	TokenizerID         string    `json:"tokenizer_id"`
	TokenizerHash       string    `json:"tokenizer_hash"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	ReasoningTokens     int64     `json:"reasoning_tokens"`
	VerifiedInputTokens int64     `json:"verified_input_tokens"`
	VerifiedOutputTokens int64    `json:"verified_output_tokens"`
	RequestHash         string    `json:"request_hash"`
	ResponseHash        string    `json:"response_hash"`
	TokenBoundaryRoot   string    `json:"token_boundary_root"`
	RequestID           string    `json:"request_id"`
	Timestamp           time.Time `json:"timestamp"`
	SignerIdentity      string    `json:"signer_identity"`
	SignerType          string    `json:"signer_type"`
	SignatureHex        string    `json:"signature_hex"`
	CertificatePEM      string    `json:"certificate_pem,omitempty"`
	RekorLogIndex       int64     `json:"rekor_log_index,omitempty"`
	Verified            bool      `json:"verified"`
	DiscrepancyPct      float64   `json:"discrepancy_pct"`

	// OMS model identity fields (schema v1.1.1) — all optional.
	ModelSignerIdentity    string `json:"model_signer_identity,omitempty"`
	ModelSignatureVerified bool   `json:"model_signature_verified,omitempty"`
}
