package api

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/yasserrmd/siqlah/internal/witness"
	"golang.org/x/mod/sumdb/note"
)

// handleC2SPCheckpoint serves the latest checkpoint in C2SP signed-note format.
// Returns text/plain (the note bytes) for witnesses to poll.
// GET /v1/witness/checkpoint
func (s *Server) handleC2SPCheckpoint(w http.ResponseWriter, r *http.Request) {
	cp, err := s.store.LatestCheckpoint()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch latest checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeError(w, http.StatusNotFound, "no checkpoints yet")
		return
	}

	rootBytes, err := hex.DecodeString(cp.RootHex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decode root hex: "+err.Error())
		return
	}

	body := witness.FormatCheckpoint(s.logOrigin, uint64(cp.TreeSize), rootBytes)

	signer, err := witness.NewNoteSigner(s.logOrigin, s.operatorPriv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create note signer: "+err.Error())
		return
	}

	signed, err := witness.SignCheckpoint(body, signer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sign checkpoint: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(signed)
}

// handleC2SPCosign accepts a cosigned checkpoint from a witness.
// POST /v1/witness/cosign  body: raw cosigned note text
func (s *Server) handleC2SPCosign(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "text/plain" && r.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be text/plain")
		return
	}

	raw := make([]byte, 1<<16) // 64 KiB max
	n, err := r.Body.Read(raw)
	if err != nil && n == 0 {
		writeError(w, http.StatusBadRequest, "empty body")
		return
	}
	rawStr := string(raw[:n])

	// Parse the checkpoint body (without verifying signatures here — the caller is a witness).
	cp, err := witness.ParseCheckpoint(rawStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse checkpoint: "+err.Error())
		return
	}

	// Verify the operator's own signature is still intact.
	operatorVerifier, err := witness.NewNoteVerifier(s.logOrigin, s.operatorPub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create operator verifier: "+err.Error())
		return
	}
	if _, _, err := witness.OpenCheckpoint(rawStr, note.VerifierList(operatorVerifier)); err != nil {
		writeError(w, http.StatusBadRequest, "operator signature invalid: "+err.Error())
		return
	}

	rootHex := fmt.Sprintf("%x", cp.RootHash)

	if err := s.store.StoreCosignature(rootHex, rawStr); err != nil {
		slog.Warn("persist cosignature failed", "root_hex", rootHex, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

// handleC2SPCosignedCheckpoint returns the latest checkpoint with all stored cosignatures.
// GET /v1/witness/cosigned-checkpoint
func (s *Server) handleC2SPCosignedCheckpoint(w http.ResponseWriter, r *http.Request) {
	cp, err := s.store.LatestCheckpoint()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch latest checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeError(w, http.StatusNotFound, "no checkpoints yet")
		return
	}

	rootBytes, err := hex.DecodeString(cp.RootHex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decode root hex: "+err.Error())
		return
	}

	body := witness.FormatCheckpoint(s.logOrigin, uint64(cp.TreeSize), rootBytes)

	signer, err := witness.NewNoteSigner(s.logOrigin, s.operatorPriv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create note signer: "+err.Error())
		return
	}

	signed, err := witness.SignCheckpoint(body, signer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sign checkpoint: "+err.Error())
		return
	}

	rootHex := fmt.Sprintf("%x", rootBytes)
	cosigs, err := s.store.GetCosignatures(rootHex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch cosignatures: "+err.Error())
		return
	}

	// Build a combined note: operator sig + all witness cosigs appended.
	// The combined note text is the same body with all signatures from all notes.
	operatorVerifier, _ := witness.NewNoteVerifier(s.logOrigin, s.operatorPub)
	combined, err := witness.MergeCosignatures(signed, cosigs, operatorVerifier)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "merge cosignatures: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(combined)
}
