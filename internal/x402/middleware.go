package x402

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	PaymentHeader  = "X-Payment"
	x402Version    = "0.1"
)

type contextKey struct{}

// NewPaymentRequired builds the 402 response body for the given accepted schemes.
func NewPaymentRequired(requestID string, schemes []PaymentScheme) *PaymentRequired {
	return &PaymentRequired{
		Version:   x402Version,
		Accepts:   schemes,
		RequestID: requestID,
	}
}

// ExtractPaymentAuth reads and decodes the X-Payment header (base64 JSON).
func ExtractPaymentAuth(r *http.Request) (*PaymentAuthorization, error) {
	raw := r.Header.Get(PaymentHeader)
	if raw == "" {
		return nil, fmt.Errorf("missing %s header", PaymentHeader)
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// Try raw JSON fallback (for curl-friendly usage).
		decoded = []byte(raw)
	}
	var auth PaymentAuthorization
	if err := json.Unmarshal(decoded, &auth); err != nil {
		return nil, fmt.Errorf("decode payment header: %w", err)
	}
	return &auth, nil
}

// WithPaymentAuth stores the decoded PaymentAuthorization in the request context.
func WithPaymentAuth(ctx context.Context, auth *PaymentAuthorization) context.Context {
	return context.WithValue(ctx, contextKey{}, auth)
}

// PaymentAuthFromContext retrieves the PaymentAuthorization from context.
func PaymentAuthFromContext(ctx context.Context) (*PaymentAuthorization, bool) {
	auth, ok := ctx.Value(contextKey{}).(*PaymentAuthorization)
	return auth, ok
}

// PaymentMiddleware gates a handler behind x402 payment verification.
// If X-Payment is absent or invalid, it responds with HTTP 402 and the provided schemes.
func PaymentMiddleware(next http.Handler, schemes []PaymentScheme) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, err := ExtractPaymentAuth(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			pr := NewPaymentRequired(r.Header.Get("X-Request-Id"), schemes)
			_ = json.NewEncoder(w).Encode(pr)
			return
		}
		ctx := WithPaymentAuth(r.Context(), auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
