package api

import (
	"net/http"
	"strconv"

	"github.com/yasserrmd/siqlah/internal/checkpoint"
)

// WitnessRequest is the request body for POST /v1/checkpoints/{id}/witness.
type WitnessRequest struct {
	WitnessID string `json:"witness_id"`
	SigHex    string `json:"sig_hex"`
}

// CheckpointVerifyResponse is returned by GET /v1/checkpoints/{id}/verify.
type CheckpointVerifyResponse struct {
	OperatorValid bool              `json:"operator_valid"`
	OperatorError string            `json:"operator_error,omitempty"`
	Witnesses     map[string]string `json:"witnesses"`
}

func (s *Server) handleBuildCheckpoint(w http.ResponseWriter, r *http.Request) {
	cp, err := s.builder.BuildAndSign()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no unbatched receipts"})
		return
	}
	writeJSON(w, http.StatusCreated, cp)
}

func (s *Server) handleListCheckpoints(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")
	offset, _ := strconv.Atoi(offsetStr)
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cps, err := s.store.ListCheckpoints(offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list checkpoints: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checkpoints": cps, "offset": offset, "limit": limit})
}

func (s *Server) handleGetCheckpoint(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	cp, err := s.store.GetCheckpoint(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeError(w, http.StatusNotFound, "checkpoint not found")
		return
	}
	writeJSON(w, http.StatusOK, cp)
}

func (s *Server) handleVerifyCheckpoint(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	cp, err := s.store.GetCheckpoint(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeError(w, http.StatusNotFound, "checkpoint not found")
		return
	}

	resp := CheckpointVerifyResponse{}
	if err := checkpoint.VerifyOperatorSignature(*cp, s.operatorPub); err != nil {
		resp.OperatorValid = false
		resp.OperatorError = err.Error()
	} else {
		resp.OperatorValid = true
	}

	witnesses, err := s.store.WitnessSignatures(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch witnesses: "+err.Error())
		return
	}
	resp.Witnesses = witnesses
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWitnessSubmit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	cp, err := s.store.GetCheckpoint(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeError(w, http.StatusNotFound, "checkpoint not found")
		return
	}

	var req WitnessRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.WitnessID == "" || req.SigHex == "" {
		writeError(w, http.StatusBadRequest, "witness_id and sig_hex are required")
		return
	}

	if err := s.store.AddWitnessSignature(id, req.WitnessID, req.SigHex); err != nil {
		writeError(w, http.StatusInternalServerError, "store witness sig: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "accepted"})
}

// parseID extracts a named path value as int64, writing a 400/404 on failure.
func parseID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, name+" must be a positive integer")
		return 0, false
	}
	return id, true
}
