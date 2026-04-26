package api

import (
	"net/http"

	"github.com/yasserrmd/siqlah/internal/x402"
)

// handleIngestWithPayment is POST /v1/receipts/with-payment.
// Requires an X-Payment header with a base64-encoded PaymentAuthorization JSON.
// Returns HTTP 402 with accepted payment schemes if the header is absent.
func (s *Server) handleIngestWithPayment(w http.ResponseWriter, r *http.Request) {
	auth, err := x402.ExtractPaymentAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		pr := x402.NewPaymentRequired(r.Header.Get("X-Request-Id"), s.paymentSchemes())
		writeJSON(w, http.StatusPaymentRequired, pr)
		return
	}

	var req IngestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	receipt, err := s.buildReceipt(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.store.AppendReceipt(*receipt); err != nil {
		writeError(w, http.StatusInternalServerError, "store receipt: "+err.Error())
		return
	}

	s.x402Bridge.Store(receipt.ID, auth)
	resp := s.x402Bridge.WrapReceipt(receipt, auth)
	writeJSON(w, http.StatusCreated, resp)
}

// handleGetReceiptPayment is GET /v1/receipts/{id}/payment.
// Returns the PaymentResponse for a receipt that was created via /v1/receipts/with-payment.
func (s *Server) handleGetReceiptPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sr, err := s.store.GetReceiptByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch receipt: "+err.Error())
		return
	}
	if sr == nil {
		writeError(w, http.StatusNotFound, "receipt not found")
		return
	}

	auth, ok := s.x402Bridge.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "no payment record for this receipt")
		return
	}

	resp := s.x402Bridge.WrapReceipt(&sr.Receipt, auth)
	writeJSON(w, http.StatusOK, resp)
}

// paymentSchemes returns the x402 payment schemes accepted by this operator.
// Recipient is set via --x402-recipient; the zero address is returned when unconfigured
// so that operators must explicitly opt-in before real payments can be routed.
func (s *Server) paymentSchemes() []x402.PaymentScheme {
	recipient := s.x402Recipient
	if recipient == "" {
		recipient = "0x0000000000000000000000000000000000000000"
	}
	return []x402.PaymentScheme{
		{
			Scheme:    "x402/evm-token",
			Network:   "base-mainnet",
			Amount:    "1000000", // 1 USDC (6 decimals)
			Token:     "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913", // USDC on Base
			Recipient: recipient,
		},
	}
}
