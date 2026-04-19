// Package attestation wraps siqlah receipts as in-toto v1 Statements, enabling
// integration with SLSA-aware tooling (GUAC, cosign attest, policy engines).
package attestation

import (
	"encoding/json"
	"fmt"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// in-toto v1 statement and predicate type URIs.
const (
	StatementTypeURI  = "https://in-toto.io/Statement/v1"
	PredicateTypeURI  = "https://siqlah.dev/receipt/v1"
)

// Statement is a JSON-serializable in-toto v1 Statement.
// Fields follow https://github.com/in-toto/attestation/tree/main/spec/v1.
type Statement struct {
	Type          string             `json:"_type"`
	Subject       []ResourceDigest   `json:"subject"`
	PredicateType string             `json:"predicateType"`
	Predicate     any                `json:"predicate"`
}

// ResourceDigest identifies an artifact by name and a map of digest algorithms to values.
type ResourceDigest struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// ReceiptStatement wraps a siqlah receipt as an in-toto v1 Statement.
type ReceiptStatement struct {
	stmt *Statement
}

// NewReceiptStatement builds an in-toto Statement from a siqlah receipt.
// The statement subjects are the request hash and response hash, which are
// the verifiable artifacts of the AI API call. The predicate is the receipt
// itself plus optional model and energy provenance data.
func NewReceiptStatement(receipt *vur.Receipt) (*ReceiptStatement, error) {
	if receipt == nil {
		return nil, fmt.Errorf("receipt must not be nil")
	}
	if receipt.RequestHash == "" || receipt.ResponseHash == "" {
		return nil, fmt.Errorf("receipt %s is missing request or response hash", receipt.ID)
	}

	subjects := []ResourceDigest{
		{
			Name:   "request:" + receipt.ID,
			Digest: map[string]string{"sha256": receipt.RequestHash},
		},
		{
			Name:   "response:" + receipt.ID,
			Digest: map[string]string{"sha256": receipt.ResponseHash},
		},
	}

	predicate := buildPredicate(receipt)

	return &ReceiptStatement{
		stmt: &Statement{
			Type:          StatementTypeURI,
			Subject:       subjects,
			PredicateType: PredicateTypeURI,
			Predicate:     predicate,
		},
	}, nil
}

// Bytes serializes the Statement as JSON.
func (rs *ReceiptStatement) Bytes() ([]byte, error) {
	return json.Marshal(rs.stmt)
}
