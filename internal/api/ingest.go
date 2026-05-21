package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yasserrmd/siqlah/internal/energy"
	"github.com/yasserrmd/siqlah/internal/model"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// IngestRequest is the request body for POST /v1/receipts.
type IngestRequest struct {
	Provider             string          `json:"provider"`
	Tenant               string          `json:"tenant"`
	Model                string          `json:"model"`
	ResponseBody         json.RawMessage `json:"response_body"`
	RequestBody          json.RawMessage `json:"request_body,omitempty"`
	RequestID            string          `json:"request_id,omitempty"`
	ModelSignatureBundle string          `json:"model_signature_bundle,omitempty"` // OMS bundle JSON
}

// IngestBatchRequest is the request body for POST /v1/receipts/batch.
type IngestBatchRequest struct {
	Items []IngestRequest `json:"items"`
}

// validateIngestRequest returns an error if any required field is missing.
func validateIngestRequest(req IngestRequest) error {
	if req.Provider == "" {
		return errors.New("provider is required")
	}
	if req.Model == "" {
		return errors.New("model is required")
	}
	if len(req.ResponseBody) == 0 {
		return errors.New("response_body is required")
	}
	return nil
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateIngestRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MiB
	var req IngestBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 10 MiB limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	const maxBatchItems = 500
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items must not be empty")
		return
	}
	if len(req.Items) > maxBatchItems {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("batch exceeds maximum of %d items", maxBatchItems))
		return
	}
	receipts := make([]*vur.Receipt, 0, len(req.Items))
	built := make([]vur.Receipt, 0, len(req.Items))
	for _, item := range req.Items {
		if err := validateIngestRequest(item); err != nil {
			writeError(w, http.StatusBadRequest, "item error: "+err.Error())
			return
		}
		receipt, err := s.buildReceipt(item)
		if err != nil {
			writeError(w, http.StatusBadRequest, "item error: "+err.Error())
			return
		}
		receipts = append(receipts, receipt)
		built = append(built, *receipt)
	}
	if _, err := s.store.AppendReceiptsBatch(built); err != nil {
		writeError(w, http.StatusInternalServerError, "store receipts: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"count": len(receipts), "receipts": receipts})
}

// ListReceiptsResponse is returned by GET /v1/receipts.
type ListReceiptsResponse struct {
	Receipts []vur.Receipt `json:"receipts"`
	Offset   int           `json:"offset"`
	Limit    int           `json:"limit"`
	Count    int           `json:"count"`
}

func (s *Server) handleListReceipts(w http.ResponseWriter, r *http.Request) {
	// Range query for the monitor daemon: GET /v1/receipts?batch_start=X&batch_end=Y
	if bs := r.URL.Query().Get("batch_start"); bs != "" {
		batchStart, err := strconv.ParseInt(bs, 10, 64)
		if err != nil || batchStart <= 0 {
			writeError(w, http.StatusBadRequest, "batch_start must be a positive integer")
			return
		}
		batchEnd, err := strconv.ParseInt(r.URL.Query().Get("batch_end"), 10, 64)
		if err != nil || batchEnd <= 0 {
			writeError(w, http.StatusBadRequest, "batch_end must be a positive integer")
			return
		}
		if batchStart > batchEnd {
			writeError(w, http.StatusBadRequest, "batch_start must not exceed batch_end")
			return
		}
		receipts, err := s.store.GetReceiptsByRange(batchStart, batchEnd)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "fetch receipts: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"receipts": receipts})
		return
	}

	const defaultLimit = 50
	const maxLimit = 500

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}

	stored, err := s.store.ListReceipts(offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list receipts: "+err.Error())
		return
	}
	receipts := make([]vur.Receipt, len(stored))
	for i, sr := range stored {
		receipts[i] = sr.Receipt
	}
	writeJSON(w, http.StatusOK, ListReceiptsResponse{
		Receipts: receipts,
		Offset:   offset,
		Limit:    limit,
		Count:    len(receipts),
	})
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
		ID:              uuid.New().String(),
		Version:         "1.1.0",
		Tenant:          req.Tenant,
		Provider:        req.Provider,
		Model:           req.Model,
		InputTokens:     usage.InputTokens,
		OutputTokens:    usage.OutputTokens,
		ReasoningTokens: usage.ReasoningTokens,
		RequestHash:     reqHash,
		ResponseHash:    respHash,
		RequestID:       reqID,
		Timestamp:       time.Now().UTC(),
		SignerIdentity:  hex.EncodeToString(s.operatorPub),
	}

	// Populate OMS model identity fields (graceful degradation on failure).
	s.populateModelIdentity(receipt, req.ModelSignatureBundle)
	s.populateEnergyFields(receipt)

	if err := vur.SignReceipt(receipt, s.operatorPriv); err != nil {
		return nil, err
	}
	receipt.Verified = true
	return receipt, nil
}

// populateModelIdentity resolves OMS model identity for a receipt.
// If a bundle is provided it is verified; otherwise the registry is consulted.
// Errors are logged but do not fail the ingestion.
func (s *Server) populateModelIdentity(r *vur.Receipt, bundleJSON string) {
	if bundleJSON != "" {
		bundleJSON, _ = model.ParseBundleFromPEM(bundleJSON)
		id, err := model.VerifyModelIdentity(bundleJSON, r.Model, model.VerifyOptions{})
		if err != nil {
			slog.Warn("OMS bundle verification failed", "model", r.Model, "error", err)
			return
		}
		r.ModelSignerIdentity = id.SignerIdentity
		r.ModelSignatureVerified = id.Verified
		if id.ModelDigest != "" && r.ModelDigest == "" {
			r.ModelDigest = id.ModelDigest
		}
		return
	}

	// Fall back to registry lookup.
	id, err := s.modelReg.Lookup(r.Model)
	if err != nil || id == nil {
		return
	}
	r.ModelSignerIdentity = id.SignerIdentity
	r.ModelSignatureVerified = id.Verified
}

// populateEnergyFields estimates energy and carbon footprint for the receipt.
func (s *Server) populateEnergyFields(r *vur.Receipt) {
	est, err := s.energyEst.Estimate(r.Model, r.InputTokens, r.OutputTokens)
	if err != nil || est.Source == energy.SourceNone {
		r.EnergySource = energy.SourceNone
		return
	}
	r.EnergyEstimateJoules = est.Joules
	r.EnergySource = est.Source
	if s.inferenceRegion != "" {
		intensity, err := s.carbonLookup.Intensity(s.inferenceRegion)
		if err == nil {
			r.CarbonIntensityGCO2ePerKWh = intensity
			r.InferenceRegion = s.inferenceRegion
		}
	}
}

func hashBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
