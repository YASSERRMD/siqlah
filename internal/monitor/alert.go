package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Alerter is implemented by any type that can receive discrepancy alerts.
type Alerter interface {
	Alert(a DiscrepancyAlert)
}

// LogAlerter logs alerts to the standard logger.
type LogAlerter struct{}

// Alert logs the discrepancy to stdout.
func (l *LogAlerter) Alert(a DiscrepancyAlert) {
	log.Printf("DISCREPANCY receipt=%s provider=%s model=%s tenant=%s "+
		"in_provider=%d in_verified=%d in_pct=%.2f%% "+
		"out_provider=%d out_verified=%d out_pct=%.2f%% threshold=%.1f%%",
		a.ReceiptID, a.Provider, a.Model, a.Tenant,
		a.ProviderInputTokens, a.VerifiedInputTokens, a.InputDiscrepancyPct,
		a.ProviderOutputTokens, a.VerifiedOutputTokens, a.OutputDiscrepancyPct,
		a.Threshold,
	)
}

// WebhookAlerter POSTs discrepancy alerts to a webhook URL as JSON.
type WebhookAlerter struct {
	URL     string
	client  *http.Client
	Headers map[string]string
}

// NewWebhookAlerter creates a WebhookAlerter with the given URL.
func NewWebhookAlerter(url string) *WebhookAlerter {
	return &WebhookAlerter{
		URL:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type webhookPayload struct {
	Event     string           `json:"event"`
	Timestamp string           `json:"timestamp"`
	Alert     DiscrepancyAlert `json:"alert"`
}

// Alert POSTs the alert as JSON to the webhook URL.
func (w *WebhookAlerter) Alert(a DiscrepancyAlert) {
	payload := webhookPayload{
		Event:     "discrepancy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Alert:     a,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook alert: marshal: %v", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook alert: create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		log.Printf("webhook alert: send: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("webhook alert: server returned %d", resp.StatusCode)
	}
}

// MultiAlerter fans out an alert to multiple Alerters.
type MultiAlerter struct {
	alerters []Alerter
}

// NewMultiAlerter creates a MultiAlerter that fans out to all given alerters.
func NewMultiAlerter(alerters ...Alerter) *MultiAlerter {
	return &MultiAlerter{alerters: alerters}
}

// Alert dispatches to all registered alerters.
func (m *MultiAlerter) Alert(a DiscrepancyAlert) {
	for _, al := range m.alerters {
		al.Alert(a)
	}
}

// MemoryAlerter captures alerts in memory for testing.
type MemoryAlerter struct {
	Alerts []DiscrepancyAlert
}

// Alert appends the alert to the in-memory slice.
func (m *MemoryAlerter) Alert(a DiscrepancyAlert) {
	m.Alerts = append(m.Alerts, a)
}

// String returns a human-readable summary of the alert.
func (a DiscrepancyAlert) String() string {
	return fmt.Sprintf("receipt=%s provider=%s in_pct=%.2f%% out_pct=%.2f%%",
		a.ReceiptID, a.Provider, a.InputDiscrepancyPct, a.OutputDiscrepancyPct)
}
