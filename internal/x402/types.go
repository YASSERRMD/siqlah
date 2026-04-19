// Package x402 implements the HTTP 402 Payment Required protocol for usage receipts.
package x402

import "github.com/yasserrmd/siqlah/pkg/vur"

// PaymentRequired is returned with HTTP 402 when payment must be submitted.
type PaymentRequired struct {
	Version   string          `json:"version"`
	Accepts   []PaymentScheme `json:"accepts"`
	RequestID string          `json:"request_id,omitempty"`
}

// PaymentScheme describes one accepted payment method.
type PaymentScheme struct {
	Scheme    string `json:"scheme"`    // e.g. "x402/evm-token"
	Network   string `json:"network"`   // e.g. "base-mainnet"
	Amount    string `json:"amount"`    // smallest units, e.g. "1000000" for 1 USDC
	Token     string `json:"token"`     // ERC-20 contract address
	Recipient string `json:"recipient"` // operator wallet address
}

// PaymentAuthorization is submitted by the client to authorize payment.
// Sent in the X-Payment header as base64-encoded JSON.
type PaymentAuthorization struct {
	Scheme      string `json:"scheme"`
	Network     string `json:"network"`
	TxHash      string `json:"tx_hash"`
	FromAddress string `json:"from_address"`
	Amount      string `json:"amount"`
	Token       string `json:"token"`
	Recipient   string `json:"recipient"`
	SignedAt    string `json:"signed_at"` // RFC3339
}

// PaymentResponse wraps a receipt with its associated payment record.
type PaymentResponse struct {
	Receipt        *vur.Receipt          `json:"receipt"`
	Payment        *PaymentAuthorization `json:"payment"`
	PaymentVerified bool                 `json:"payment_verified"`
	PaymentError   string                `json:"payment_error,omitempty"`
}
