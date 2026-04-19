package monitor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

func TestDiscrepancyFunction(t *testing.T) {
	tests := []struct {
		provider, verified int64
		want               float64
	}{
		{100, 100, 0},
		{100, 90, 10},
		{100, 110, 10},
		{0, 0, 0},
		{100, 0, 100},
	}
	for _, tt := range tests {
		got := discrepancy(tt.provider, tt.verified)
		if got != tt.want {
			t.Errorf("discrepancy(%d,%d)=%.2f, want %.2f", tt.provider, tt.verified, got, tt.want)
		}
	}
}

func TestCheckReceiptNoAlert(t *testing.T) {
	mem := &MemoryAlerter{}
	m := &Monitor{alerter: mem, threshold: 5.0}

	// Receipt with no verified tokens — should not alert.
	r := vur.Receipt{ID: "r1", InputTokens: 100, OutputTokens: 50}
	m.checkReceipt(context.Background(), r)
	if len(mem.Alerts) != 0 {
		t.Errorf("expected no alerts, got %d", len(mem.Alerts))
	}
}

func TestCheckReceiptBelowThreshold(t *testing.T) {
	mem := &MemoryAlerter{}
	m := &Monitor{alerter: mem, threshold: 10.0}

	// 5% discrepancy, threshold 10% — should not alert.
	r := vur.Receipt{
		ID:                   "r2",
		InputTokens:          100,
		VerifiedInputTokens:  95,
		OutputTokens:         50,
		VerifiedOutputTokens: 50,
	}
	m.checkReceipt(context.Background(), r)
	if len(mem.Alerts) != 0 {
		t.Errorf("expected no alert, got %d", len(mem.Alerts))
	}
}

func TestCheckReceiptAboveThreshold(t *testing.T) {
	mem := &MemoryAlerter{}
	m := &Monitor{alerter: mem, threshold: 5.0}

	// 20% input discrepancy — should alert.
	r := vur.Receipt{
		ID:                   "r3",
		Provider:             "openai",
		InputTokens:          100,
		VerifiedInputTokens:  80,
		OutputTokens:         50,
		VerifiedOutputTokens: 50,
	}
	m.checkReceipt(context.Background(), r)
	if len(mem.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(mem.Alerts))
	}
	if mem.Alerts[0].InputDiscrepancyPct != 20 {
		t.Errorf("expected 20%% discrepancy, got %.2f%%", mem.Alerts[0].InputDiscrepancyPct)
	}
}

func TestThresholdFiltering(t *testing.T) {
	mem := &MemoryAlerter{}
	m := &Monitor{alerter: mem, threshold: 50.0}

	// 20% discrepancy but threshold is 50% — no alert.
	r := vur.Receipt{
		ID:                  "r4",
		InputTokens:         100,
		VerifiedInputTokens: 80,
		OutputTokens:        50,
		VerifiedOutputTokens: 50,
	}
	m.checkReceipt(context.Background(), r)
	if len(mem.Alerts) != 0 {
		t.Errorf("expected no alert above threshold, got %d", len(mem.Alerts))
	}
}

func TestWebhookAlerterSendsPayload(t *testing.T) {
	var received webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wa := NewWebhookAlerter(srv.URL)
	wa.Alert(DiscrepancyAlert{
		ReceiptID:           "r-webhook-test",
		Provider:            "openai",
		InputDiscrepancyPct: 15.5,
		Threshold:           5.0,
	})

	if received.Event != "discrepancy" {
		t.Errorf("expected event=discrepancy, got %q", received.Event)
	}
	if received.Alert.ReceiptID != "r-webhook-test" {
		t.Errorf("expected receipt_id=r-webhook-test, got %q", received.Alert.ReceiptID)
	}
	if received.Alert.InputDiscrepancyPct != 15.5 {
		t.Errorf("expected 15.5%%, got %.2f%%", received.Alert.InputDiscrepancyPct)
	}
}

func TestMultiAlerterFansOut(t *testing.T) {
	m1 := &MemoryAlerter{}
	m2 := &MemoryAlerter{}
	multi := NewMultiAlerter(m1, m2)
	alert := DiscrepancyAlert{ReceiptID: "r-multi"}
	multi.Alert(alert)
	if len(m1.Alerts) != 1 || len(m2.Alerts) != 1 {
		t.Error("expected both alerters to receive the alert")
	}
}

func TestMonitorRunWithMockLedger(t *testing.T) {
	// Two checkpoints, each with one receipt that has a discrepancy.
	receipts := []vur.Receipt{
		{
			ID:                   "r-monitor-1",
			Provider:             "openai",
			InputTokens:          100,
			VerifiedInputTokens:  70, // 30% discrepancy
			OutputTokens:         50,
			VerifiedOutputTokens: 50,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
			json.NewEncoder(w).Encode(map[string]any{"receipts": receipts})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mem := &MemoryAlerter{}
	mon := New(Config{
		LedgerURL: srv.URL,
		Alerter:   mem,
		Interval:  100 * time.Millisecond,
		Threshold: 5.0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	mon.Run(ctx)

	if len(mem.Alerts) == 0 {
		t.Error("expected at least one alert from monitor run")
	}
}

func TestLogAlerterString(t *testing.T) {
	a := DiscrepancyAlert{
		ReceiptID:           "r-str",
		Provider:            "openai",
		InputDiscrepancyPct: 12.34,
	}
	s := a.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
}
