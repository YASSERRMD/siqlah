package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// IngestRequest is the request body for POST /v1/receipts.
type IngestRequest struct {
	Provider     string          `json:"provider"`
	Tenant       string          `json:"tenant"`
	Model        string          `json:"model"`
	ResponseBody json.RawMessage `json:"response_body"`
	RequestBody  json.RawMessage `json:"request_body,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
}

// IngestBatchRequest is the request body for POST /v1/receipts/batch.
type IngestBatchRequest struct {
	Items []IngestRequest `json:"items"`
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	writeJSON(w, http.StatusCreated, receipt)
}

func (s *Server) handleIngestBatch(w http.ResponseWriter, r *http.Request) {
	var req IngestBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items must not be empty")
		return
	}
	receipts := make([]*vur.Receipt, 0, len(req.Items))
	for _, item := range req.Items {
		receipt, err := s.buildReceipt(item)
		if err != nil {
			writeError(w, http.StatusBadRequest, "item error: "+err.Error())
			return
		}
		if _, err := s.store.AppendReceipt(*receipt); err != nil {
			writeError(w, http.StatusInternalServerError, "store receipt: "+err.Error())
			return
		}
		receipts = append(receipts, receipt)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"count": len(receipts), "receipts": receipts})
}

func (s *Server) handleGetReceipt(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, sr.Receipt)
}

// buildReceipt creates, signs, and returns a Receipt from an IngestRequest.
func (s *Server) buildReceipt(req IngestRequest) (*vur.Receipt, error) {
	adapter, err := s.registry.Get(req.Provider)
	if err != nil {
		return nil, err
	}
	usage, err := adapter.ParseUsage(req.ResponseBody)
	if err != nil {
		return nil, err
	}

	reqID := req.RequestID
	if reqID == "" {
		reqID = usage.RequestID
	}

	// Hash raw request/response for tamper detection.
	reqHash := hashBytes(req.RequestBody)
	respHash := hashBytes(req.ResponseBody)

	receipt := &vur.Receipt{
		ID:           uuid.New().String(),
		Version:      "1.0.0",
		Tenant:       req.Tenant,
		Provider:     req.Provider,
		Model:        req.Model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		ReasoningTokens: usage.ReasoningTokens,
		RequestHash:  reqHash,
		ResponseHash: respHash,
		RequestID:    reqID,
		Timestamp:    time.Now().UTC(),
		SignerIdentity: hex.EncodeToString(s.operatorPub),
	}

	if err := vur.SignReceipt(receipt, s.operatorPriv); err != nil {
		return nil, err
	}
	receipt.Verified = true
	return receipt, nil
}

func hashBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
