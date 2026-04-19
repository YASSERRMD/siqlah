package api

import (
	"crypto/ed25519"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/model"
	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/internal/x402"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// Server is the HTTP API server for siqlah.
type Server struct {
	store        store.Store
	builder      *checkpoint.Builder
	operatorPub  ed25519.PublicKey
	operatorPriv ed25519.PrivateKey
	registry     providerRegistry
	version      string
	logOrigin    string
	x402Bridge   *x402.Bridge
	modelReg     *model.Registry
}

// providerRegistry abstracts the provider.Registry for test injection.
type providerRegistry interface {
	Get(name string) (vur.ProviderAdapter, error)
}

// New creates a new Server.
func New(
	st store.Store,
	b *checkpoint.Builder,
	operatorPub ed25519.PublicKey,
	operatorPriv ed25519.PrivateKey,
	reg providerRegistry,
	version string,
) *Server {
	return NewWithOrigin(st, b, operatorPub, operatorPriv, reg, version, "")
}

// NewWithOrigin creates a new Server with a custom C2SP log origin string.
func NewWithOrigin(
	st store.Store,
	b *checkpoint.Builder,
	operatorPub ed25519.PublicKey,
	operatorPriv ed25519.PrivateKey,
	reg providerRegistry,
	version string,
	logOrigin string,
) *Server {
	if version == "" {
		version = "dev"
	}
	if logOrigin == "" {
		logOrigin = "siqlah.dev/log"
	}
	return &Server{
		store:        st,
		builder:      b,
		operatorPub:  operatorPub,
		operatorPriv: operatorPriv,
		registry:     reg,
		version:      version,
		logOrigin:    logOrigin,
		x402Bridge:   x402.NewBridge(),
		modelReg:     model.NewRegistry(),
	}
}

// Routes returns a ServeMux with all API routes registered.
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Receipt routes
	mux.HandleFunc("POST /v1/receipts", s.handleIngest)
	mux.HandleFunc("POST /v1/receipts/batch", s.handleIngestBatch)
	mux.HandleFunc("GET /v1/receipts/{id}", s.handleGetReceipt)
	mux.HandleFunc("GET /v1/receipts/{id}/proof", s.handleInclusionProof)

	// Checkpoint routes
	mux.HandleFunc("POST /v1/checkpoints/build", s.handleBuildCheckpoint)
	mux.HandleFunc("GET /v1/checkpoints", s.handleListCheckpoints)
	mux.HandleFunc("GET /v1/checkpoints/{id}", s.handleGetCheckpoint)
	mux.HandleFunc("GET /v1/checkpoints/{id}/verify", s.handleVerifyCheckpoint)
	mux.HandleFunc("POST /v1/checkpoints/{id}/witness", s.handleWitnessSubmit)
	mux.HandleFunc("GET /v1/checkpoints/{id}/consistency/{old_id}", s.handleConsistencyProof)

	// Tessera log routes (available when Tessera backend is configured)
	mux.HandleFunc("GET /v1/log/checkpoint", s.handleLogCheckpoint)

	// C2SP witness routes
	mux.HandleFunc("GET /v1/witness/checkpoint", s.handleC2SPCheckpoint)
	mux.HandleFunc("POST /v1/witness/cosign", s.handleC2SPCosign)
	mux.HandleFunc("GET /v1/witness/cosigned-checkpoint", s.handleC2SPCosignedCheckpoint)

	// x402 payment routes
	mux.HandleFunc("POST /v1/receipts/with-payment", s.handleIngestWithPayment)
	mux.HandleFunc("GET /v1/receipts/{id}/payment", s.handleGetReceiptPayment)

	// Utility routes
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/stats", s.handleStats)

	return mux
}

// Handler returns the root handler with logging and CORS middleware applied.
func (s *Server) Handler() http.Handler {
	return corsMiddleware(loggingMiddleware(s.Routes()))
}

// loggingMiddleware logs each request method, path, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.code, time.Since(start))
	})
}

// corsMiddleware adds permissive CORS headers for API development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// responseWriter captures the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
