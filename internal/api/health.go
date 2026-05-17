package api

import (
	"net/http"
	"time"
)

// HealthResponse is returned by GET /v1/health.
type HealthResponse struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	LogBackend string `json:"log_backend,omitempty"`
	SignerType  string `json:"signer_type,omitempty"`
	Timestamp  string `json:"timestamp"`
}

// StatsResponse is returned by GET /v1/stats.
type StatsResponse struct {
	TotalReceipts    int64 `json:"total_receipts"`
	TotalCheckpoints int64 `json:"total_checkpoints"`
	PendingBatch     int64 `json:"pending_batch"`
	TotalWitnessSigs int64 `json:"total_witness_sigs"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Version:    s.version,
		LogBackend: s.logBackend,
		SignerType:  s.signerType,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	// Probe DB connectivity via Stats (lightweight query).
	if _, err := s.store.Stats(); err != nil {
		resp.Status = "degraded"
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}
	resp.Status = "ok"
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch stats: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, StatsResponse{
		TotalReceipts:    stats.TotalReceipts,
		TotalCheckpoints: stats.TotalCheckpoints,
		PendingBatch:     stats.PendingBatch,
		TotalWitnessSigs: stats.TotalWitnessSigs,
	})
}
