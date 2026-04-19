package api

import (
	"net/http"

	"github.com/yasserrmd/siqlah/internal/attestation"
)

// handleAttestation serves GET /v1/receipts/{id}/attestation.
// Returns an in-toto v1 Statement wrapping the receipt as a
// "https://siqlah.dev/receipt/v1" predicate.
func (s *Server) handleAttestation(w http.ResponseWriter, r *http.Request) {
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

	stmt, err := attestation.NewReceiptStatement(&sr.Receipt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build attestation: "+err.Error())
		return
	}

	b, err := stmt.Bytes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "serialize attestation: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/vnd.in-toto+json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
