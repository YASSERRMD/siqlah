package x402

import (
	"fmt"
	"strings"
	"sync"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// Bridge stores payment authorizations and wraps receipts with payment metadata.
type Bridge struct {
	mu       sync.RWMutex
	payments map[string]*PaymentAuthorization // receipt_id → authorization
}

// NewBridge creates a Bridge with an empty payment store.
func NewBridge() *Bridge {
	return &Bridge{payments: make(map[string]*PaymentAuthorization)}
}

// Store records a payment authorization for the given receipt ID.
func (b *Bridge) Store(receiptID string, auth *PaymentAuthorization) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.payments[receiptID] = auth
}

// Get retrieves the payment authorization associated with a receipt ID.
func (b *Bridge) Get(receiptID string) (*PaymentAuthorization, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	auth, ok := b.payments[receiptID]
	return auth, ok
}

// WrapReceipt creates a PaymentResponse by pairing a receipt with its payment authorization.
// It runs structural verification and sets PaymentVerified accordingly.
func (b *Bridge) WrapReceipt(r *vur.Receipt, auth *PaymentAuthorization) *PaymentResponse {
	resp := &PaymentResponse{
		Receipt: r,
		Payment: auth,
	}
	if err := VerifyPaymentAuth(auth); err != nil {
		resp.PaymentError = err.Error()
	} else {
		resp.PaymentVerified = true
	}
	return resp
}

// VerifyPaymentAuth performs structural validation of a PaymentAuthorization.
// It does NOT verify on-chain state; that requires an external RPC provider.
func VerifyPaymentAuth(auth *PaymentAuthorization) error {
	if auth == nil {
		return fmt.Errorf("payment authorization is nil")
	}
	if auth.Scheme == "" {
		return fmt.Errorf("payment scheme is required")
	}
	if !strings.HasPrefix(auth.Scheme, "x402/") {
		return fmt.Errorf("unsupported payment scheme %q: must start with x402/", auth.Scheme)
	}
	if auth.Network == "" {
		return fmt.Errorf("payment network is required")
	}
	if auth.TxHash == "" {
		return fmt.Errorf("tx_hash is required")
	}
	if auth.FromAddress == "" {
		return fmt.Errorf("from_address is required")
	}
	if auth.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if auth.Recipient == "" {
		return fmt.Errorf("recipient is required")
	}
	return nil
}
