// Package monitor implements a discrepancy detection daemon that polls a siqlah
// ledger for new receipts and alerts when provider-reported token counts differ
// from locally re-verified counts beyond a configurable threshold.
package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// Monitor polls a siqlah ledger and fires alerts for token count discrepancies.
type Monitor struct {
	ledgerURL  string
	client     *http.Client
	alerter    Alerter
	interval   time.Duration
	threshold  float64 // discrepancy percent that triggers an alert
	lastSeenID string  // last receipt ID checked (for pagination stub)
}

// Config holds Monitor configuration.
type Config struct {
	LedgerURL string
	Alerter   Alerter
	Interval  time.Duration
	Threshold float64 // alert when |delta|/provider_count > threshold/100
}

// New creates a Monitor with the given configuration.
func New(cfg Config) *Monitor {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5.0
	}
	if cfg.Alerter == nil {
		cfg.Alerter = &LogAlerter{}
	}
	return &Monitor{
		ledgerURL: cfg.LedgerURL,
		client:    &http.Client{Timeout: 10 * time.Second},
		alerter:   cfg.Alerter,
		interval:  cfg.Interval,
		threshold: cfg.Threshold,
	}
}

// Run starts the monitoring loop and blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	log.Printf("monitor: starting, ledger=%s interval=%s threshold=%.1f%%",
		m.ledgerURL, m.interval, m.threshold)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Run once immediately, then follow the ticker.
	m.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Println("monitor: stopped")
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

// tick performs one poll-and-check cycle.
func (m *Monitor) tick(ctx context.Context) {
	stats, err := m.fetchStats(ctx)
	if err != nil {
		log.Printf("monitor: fetch stats: %v", err)
		return
	}
	log.Printf("monitor: ledger total_receipts=%d pending_batch=%d",
		stats.TotalReceipts, stats.PendingBatch)

	checkpoints, err := m.fetchCheckpoints(ctx, 0, 50)
	if err != nil {
		log.Printf("monitor: fetch checkpoints: %v", err)
		return
	}
	if len(checkpoints) == 0 {
		return
	}

	for _, cp := range checkpoints {
		receipts, err := m.fetchReceiptRange(ctx, cp.BatchStart, cp.BatchEnd)
		if err != nil {
			log.Printf("monitor: fetch receipts for cp %d: %v", cp.ID, err)
			continue
		}
		for _, r := range receipts {
			m.checkReceipt(ctx, r)
		}
	}
}

// checkReceipt compares provider-reported vs verified token counts.
func (m *Monitor) checkReceipt(ctx context.Context, r vur.Receipt) {
	if r.VerifiedInputTokens == 0 && r.VerifiedOutputTokens == 0 {
		return // no local verification available
	}

	inDisc := discrepancy(r.InputTokens, r.VerifiedInputTokens)
	outDisc := discrepancy(r.OutputTokens, r.VerifiedOutputTokens)

	if inDisc > m.threshold || outDisc > m.threshold {
		m.alerter.Alert(DiscrepancyAlert{
			ReceiptID:            r.ID,
			Provider:             r.Provider,
			Model:                r.Model,
			Tenant:               r.Tenant,
			ProviderInputTokens:  r.InputTokens,
			VerifiedInputTokens:  r.VerifiedInputTokens,
			InputDiscrepancyPct:  inDisc,
			ProviderOutputTokens: r.OutputTokens,
			VerifiedOutputTokens: r.VerifiedOutputTokens,
			OutputDiscrepancyPct: outDisc,
			Threshold:            m.threshold,
		})
	}
}

// DiscrepancyAlert is the payload sent to an Alerter.
type DiscrepancyAlert struct {
	ReceiptID            string
	Provider             string
	Model                string
	Tenant               string
	ProviderInputTokens  int64
	VerifiedInputTokens  int64
	InputDiscrepancyPct  float64
	ProviderOutputTokens int64
	VerifiedOutputTokens int64
	OutputDiscrepancyPct float64
	Threshold            float64
}

// --- HTTP helpers ---

type statsResp struct {
	TotalReceipts    int64 `json:"total_receipts"`
	TotalCheckpoints int64 `json:"total_checkpoints"`
	PendingBatch     int64 `json:"pending_batch"`
}

type checkpointRef struct {
	ID         int64 `json:"ID"`
	BatchStart int64 `json:"BatchStart"`
	BatchEnd   int64 `json:"BatchEnd"`
	TreeSize   int   `json:"TreeSize"`
}

type checkpointsListResp struct {
	Checkpoints []checkpointRef `json:"checkpoints"`
}

type receiptsListResp struct {
	Receipts []vur.Receipt `json:"receipts"`
}

func (m *Monitor) fetchStats(ctx context.Context) (*statsResp, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, m.ledgerURL+"/v1/stats", nil)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var s statsResp
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *Monitor) fetchCheckpoints(ctx context.Context, offset, limit int) ([]checkpointRef, error) {
	url := fmt.Sprintf("%s/v1/checkpoints?offset=%d&limit=%d", m.ledgerURL, offset, limit)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var cl checkpointsListResp
	if err := json.Unmarshal(body, &cl); err != nil {
		return nil, fmt.Errorf("parse checkpoints: %w (body: %s)", err, body)
	}
	return cl.Checkpoints, nil
}

func (m *Monitor) fetchReceiptRange(ctx context.Context, start, end int64) ([]vur.Receipt, error) {
	url := fmt.Sprintf("%s/v1/receipts?batch_start=%d&batch_end=%d", m.ledgerURL, start, end)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rl receiptsListResp
	if err := json.NewDecoder(resp.Body).Decode(&rl); err != nil {
		return nil, err
	}
	return rl.Receipts, nil
}

// discrepancy returns the absolute percentage difference between provider and verified counts.
func discrepancy(provider, verified int64) float64 {
	if provider == 0 {
		if verified == 0 {
			return 0
		}
		return 100
	}
	d := float64(provider - verified)
	if d < 0 {
		d = -d
	}
	return d / float64(provider) * 100
}
