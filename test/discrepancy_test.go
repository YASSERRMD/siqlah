package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/monitor"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func TestDiscrepancyDetection(t *testing.T) {
	// Receipt with inflated provider tokens (1500 reported, 1200 verified).
	r := vur.Receipt{
		ID:                   "disc-test-001",
		Provider:             "openai",
		Model:                "gpt-4o",
		Tenant:               "test-tenant",
		InputTokens:          1500,
		VerifiedInputTokens:  1200,
		OutputTokens:         500,
		VerifiedOutputTokens: 500,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/stats":
			json.NewEncoder(w).Encode(map[string]any{
				"total_receipts":    1,
				"total_checkpoints": 1,
				"pending_batch":     0,
			})
		case "/v1/checkpoints":
			json.NewEncoder(w).Encode(map[string]any{
				"checkpoints": []map[string]any{
					{"ID": 1, "BatchStart": 1, "BatchEnd": 1, "TreeSize": 1},
				},
			})
		case "/v1/receipts":
			json.NewEncoder(w).Encode(map[string]any{
				"receipts": []vur.Receipt{r},
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer srv.Close()

	mem := &monitor.MemoryAlerter{}
	mon := monitor.New(monitor.Config{
		LedgerURL: srv.URL,
		Alerter:   mem,
		Interval:  50 * time.Millisecond,
		Threshold: 5.0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	mon.Run(ctx)

	if len(mem.Alerts) == 0 {
		t.Fatal("expected discrepancy alert, got none")
	}

	alert := mem.Alerts[0]
	if alert.ReceiptID != "disc-test-001" {
		t.Errorf("expected receipt disc-test-001, got %q", alert.ReceiptID)
	}
	// 300/1500 = 20%
	if alert.InputDiscrepancyPct < 19 || alert.InputDiscrepancyPct > 21 {
		t.Errorf("expected ~20%% discrepancy, got %.2f%%", alert.InputDiscrepancyPct)
	}
	t.Logf("alert: %s", alert.String())
}

func TestDiscrepancyBelowThreshold(t *testing.T) {
	// 2% discrepancy, threshold 5% — no alert expected.
	r := vur.Receipt{
		ID:                   "disc-test-002",
		Provider:             "openai",
		InputTokens:          1000,
		VerifiedInputTokens:  980, // 2% discrepancy
		OutputTokens:         500,
		VerifiedOutputTokens: 490,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/stats":
			json.NewEncoder(w).Encode(map[string]any{"total_receipts": 1, "total_checkpoints": 1})
		case "/v1/checkpoints":
			json.NewEncoder(w).Encode(map[string]any{
				"checkpoints": []map[string]any{{"ID": 1, "BatchStart": 1, "BatchEnd": 1, "TreeSize": 1}},
			})
		case "/v1/receipts":
			json.NewEncoder(w).Encode(map[string]any{"receipts": []vur.Receipt{r}})
		default:
			http.NotFound(w, req)
		}
	}))
	defer srv.Close()

	mem := &monitor.MemoryAlerter{}
	mon := monitor.New(monitor.Config{
		LedgerURL: srv.URL,
		Alerter:   mem,
		Interval:  50 * time.Millisecond,
		Threshold: 5.0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	mon.Run(ctx)

	if len(mem.Alerts) > 0 {
		t.Errorf("expected no alerts below threshold, got %d", len(mem.Alerts))
	}
}

func TestNoVerifiedTokensNoAlert(t *testing.T) {
	// Receipt with no verified tokens — should never alert.
	r := vur.Receipt{
		ID:           "disc-test-003",
		Provider:     "openai",
		InputTokens:  1000,
		OutputTokens: 500,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/stats":
			json.NewEncoder(w).Encode(map[string]any{"total_receipts": 1, "total_checkpoints": 1})
		case "/v1/checkpoints":
			json.NewEncoder(w).Encode(map[string]any{
				"checkpoints": []map[string]any{{"ID": 1, "BatchStart": 1, "BatchEnd": 1, "TreeSize": 1}},
			})
		case "/v1/receipts":
			json.NewEncoder(w).Encode(map[string]any{"receipts": []vur.Receipt{r}})
		default:
			http.NotFound(w, req)
		}
	}))
	defer srv.Close()

	mem := &monitor.MemoryAlerter{}
	mon := monitor.New(monitor.Config{
		LedgerURL: srv.URL,
		Alerter:   mem,
		Interval:  50 * time.Millisecond,
		Threshold: 0.1, // very low threshold to catch any false positives
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	mon.Run(ctx)

	if len(mem.Alerts) > 0 {
		t.Errorf("expected no alerts when no verified tokens, got %d", len(mem.Alerts))
	}
}
