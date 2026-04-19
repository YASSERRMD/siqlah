package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
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
		ID:              uuid.New().String(),
		Version:         "1.0.0",
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
			log.Printf("OMS bundle verification failed for model %q: %v", r.Model, err)
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
