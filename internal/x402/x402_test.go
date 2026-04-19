package x402_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yasserrmd/siqlah/internal/x402"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func validAuth() *x402.PaymentAuthorization {
	return &x402.PaymentAuthorization{
		Scheme:      "x402/evm-token",
		Network:     "base-mainnet",
		TxHash:      "0xabc123",
		FromAddress: "0xdeadbeef",
		Amount:      "1000000",
		Token:       "0xtoken",
		Recipient:   "0xrecipient",
		SignedAt:    "2026-04-19T00:00:00Z",
	}
}

func TestVerifyPaymentAuth_Valid(t *testing.T) {
	if err := x402.VerifyPaymentAuth(validAuth()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPaymentAuth_Nil(t *testing.T) {
	if err := x402.VerifyPaymentAuth(nil); err == nil {
		t.Fatal("expected error for nil auth")
	}
}

func TestVerifyPaymentAuth_MissingFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*x402.PaymentAuthorization)
	}{
		{"empty scheme", func(a *x402.PaymentAuthorization) { a.Scheme = "" }},
		{"bad scheme prefix", func(a *x402.PaymentAuthorization) { a.Scheme = "evm-token" }},
		{"empty network", func(a *x402.PaymentAuthorization) { a.Network = "" }},
		{"empty tx_hash", func(a *x402.PaymentAuthorization) { a.TxHash = "" }},
		{"empty from_address", func(a *x402.PaymentAuthorization) { a.FromAddress = "" }},
		{"empty amount", func(a *x402.PaymentAuthorization) { a.Amount = "" }},
		{"empty recipient", func(a *x402.PaymentAuthorization) { a.Recipient = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validAuth()
			tc.mutate(a)
			if err := x402.VerifyPaymentAuth(a); err == nil {
				t.Errorf("expected validation error for %q", tc.name)
			}
		})
	}
}

func TestBridge_StoreAndGet(t *testing.T) {
	b := x402.NewBridge()
	auth := validAuth()
	b.Store("receipt-1", auth)
	got, ok := b.Get("receipt-1")
	if !ok {
		t.Fatal("expected to find stored auth")
	}
	if got.TxHash != auth.TxHash {
		t.Errorf("TxHash mismatch: got %q, want %q", got.TxHash, auth.TxHash)
	}
	_, ok = b.Get("receipt-unknown")
	if ok {
		t.Fatal("expected miss for unknown receipt")
	}
}

func TestBridge_WrapReceipt_Valid(t *testing.T) {
	b := x402.NewBridge()
	r := &vur.Receipt{ID: "r1", Model: "gpt-4o"}
	resp := b.WrapReceipt(r, validAuth())
	if !resp.PaymentVerified {
		t.Errorf("expected PaymentVerified=true, got false (err: %s)", resp.PaymentError)
	}
	if resp.Receipt.ID != "r1" {
		t.Errorf("receipt ID mismatch")
	}
}

func TestBridge_WrapReceipt_Invalid(t *testing.T) {
	b := x402.NewBridge()
	r := &vur.Receipt{ID: "r2", Model: "gpt-4o"}
	bad := validAuth()
	bad.TxHash = ""
	resp := b.WrapReceipt(r, bad)
	if resp.PaymentVerified {
		t.Error("expected PaymentVerified=false for invalid auth")
	}
	if resp.PaymentError == "" {
		t.Error("expected PaymentError to be set")
	}
}

func TestExtractPaymentAuth_Base64JSON(t *testing.T) {
	auth := validAuth()
	b, _ := json.Marshal(auth)
	encoded := base64.StdEncoding.EncodeToString(b)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set(x402.PaymentHeader, encoded)

	got, err := x402.ExtractPaymentAuth(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TxHash != auth.TxHash {
		t.Errorf("TxHash mismatch")
	}
}

func TestExtractPaymentAuth_RawJSON(t *testing.T) {
	auth := validAuth()
	b, _ := json.Marshal(auth)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set(x402.PaymentHeader, string(b))

	got, err := x402.ExtractPaymentAuth(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Network != auth.Network {
		t.Errorf("Network mismatch")
	}
}

func TestExtractPaymentAuth_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	_, err := x402.ExtractPaymentAuth(req)
	if err == nil {
		t.Fatal("expected error for missing header")
	}
}

func TestPaymentMiddleware_Returns402(t *testing.T) {
	handler := x402.PaymentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), []x402.PaymentScheme{{Scheme: "x402/evm-token", Network: "base-mainnet"}})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusPaymentRequired)
	}
	var pr x402.PaymentRequired
	if err := json.NewDecoder(rr.Body).Decode(&pr); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(pr.Accepts) == 0 {
		t.Error("expected at least one payment scheme in response")
	}
}

func TestPaymentMiddleware_PassesThrough(t *testing.T) {
	auth := validAuth()
	b, _ := json.Marshal(auth)
	encoded := base64.StdEncoding.EncodeToString(b)

	called := false
	handler := x402.PaymentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got, ok := x402.PaymentAuthFromContext(r.Context())
		if !ok || got.TxHash != auth.TxHash {
			t.Errorf("auth not in context or TxHash mismatch")
		}
		w.WriteHeader(http.StatusOK)
	}), nil)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set(x402.PaymentHeader, encoded)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
