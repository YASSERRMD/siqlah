// Package config provides YAML configuration file support for siqlah.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds all siqlah service configuration.
type Config struct {
	Addr                 string        `json:"addr"`
	DB                   string        `json:"db"`
	BatchInterval        time.Duration `json:"batch_interval"`
	MaxBatch             int           `json:"max_batch"`
	Witnesses            string        `json:"witnesses"`
	Monitor              bool          `json:"monitor"`
	MonitorInterval      time.Duration `json:"monitor_interval"`
	DiscrepancyThreshold float64       `json:"discrepancy_threshold"`
	AlertWebhook         string        `json:"alert_webhook"`
	OperatorKey          string        `json:"operator_key"`

	// Tessera backend configuration
	LogBackend         string `json:"log_backend"`          // "sqlite" (default) or "tessera"
	TesseraStoragePath string `json:"tessera_storage_path"` // path for POSIX tile storage
	TesseraLogName     string `json:"tessera_log_name"`     // C2SP log origin string

	// Signing backend: "ed25519" (default) or "fulcio" (keyless).
	SigningBackend string `json:"signing_backend"`
	// OIDCIssuer is the OIDC provider URL used for Fulcio keyless signing.
	OIDCIssuer string `json:"oidc_issuer"`
	// OIDCClientID is the OIDC client ID for interactive token flows.
	OIDCClientID string `json:"oidc_client_id"`
	// FulcioURL is the Fulcio CA endpoint for keyless signing.
	FulcioURL string `json:"fulcio_url"`
	// Rekor public anchoring
	RekorAnchor         bool          `json:"rekor_anchor"`          // enable anchoring (default false)
	RekorURL            string        `json:"rekor_url"`             // Rekor endpoint
	RekorAnchorInterval time.Duration `json:"rekor_anchor_interval"` // default 24h
}

// rawConfig mirrors Config with duration fields as strings for JSON parsing.
type rawConfig struct {
	Addr                 string  `json:"addr"`
	DB                   string  `json:"db"`
	BatchInterval        string  `json:"batch_interval"`
	MaxBatch             int     `json:"max_batch"`
	Witnesses            string  `json:"witnesses"`
	Monitor              bool    `json:"monitor"`
	MonitorInterval      string  `json:"monitor_interval"`
	DiscrepancyThreshold float64 `json:"discrepancy_threshold"`
	AlertWebhook         string  `json:"alert_webhook"`
	OperatorKey          string  `json:"operator_key"`
	LogBackend           string  `json:"log_backend"`
	TesseraStoragePath   string  `json:"tessera_storage_path"`
	TesseraLogName       string  `json:"tessera_log_name"`
	SigningBackend        string  `json:"signing_backend"`
	OIDCIssuer           string  `json:"oidc_issuer"`
	OIDCClientID         string  `json:"oidc_client_id"`
	FulcioURL            string  `json:"fulcio_url"`
	RekorAnchor          bool    `json:"rekor_anchor"`
	RekorURL             string  `json:"rekor_url"`
	RekorAnchorInterval  string  `json:"rekor_anchor_interval"`
}

// Defaults returns a Config populated with default values matching the CLI flags.
func Defaults() Config {
	return Config{
		Addr:                 ":8080",
		DB:                   "siqlah.db",
		BatchInterval:        30 * time.Second,
		MaxBatch:             1000,
		Monitor:              false,
		MonitorInterval:      60 * time.Second,
		DiscrepancyThreshold: 5.0,
		LogBackend:           "sqlite",
		TesseraStoragePath:   "./tessera-data/",
		TesseraLogName:       "siqlah.dev/log",
		SigningBackend:        "ed25519",
		OIDCIssuer:           "https://accounts.google.com",
		FulcioURL:            "https://fulcio.sigstore.dev",
		RekorAnchor:         false,
		RekorURL:            "https://rekor.sigstore.dev",
		RekorAnchorInterval: 24 * time.Hour,
	}
}

// Load reads a JSON config file from path, merging over defaults.
// Missing keys keep their default values.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if raw.Addr != "" {
		cfg.Addr = raw.Addr
	}
	if raw.DB != "" {
		cfg.DB = raw.DB
	}
	if raw.BatchInterval != "" {
		d, err := time.ParseDuration(raw.BatchInterval)
		if err != nil {
			return nil, fmt.Errorf("batch_interval: %w", err)
		}
		cfg.BatchInterval = d
	}
	if raw.MaxBatch > 0 {
		cfg.MaxBatch = raw.MaxBatch
	}
	if raw.Witnesses != "" {
		cfg.Witnesses = raw.Witnesses
	}
	cfg.Monitor = raw.Monitor
	if raw.MonitorInterval != "" {
		d, err := time.ParseDuration(raw.MonitorInterval)
		if err != nil {
			return nil, fmt.Errorf("monitor_interval: %w", err)
		}
		cfg.MonitorInterval = d
	}
	if raw.DiscrepancyThreshold > 0 {
		cfg.DiscrepancyThreshold = raw.DiscrepancyThreshold
	}
	if raw.AlertWebhook != "" {
		cfg.AlertWebhook = raw.AlertWebhook
	}
	if raw.OperatorKey != "" {
		cfg.OperatorKey = raw.OperatorKey
	}
	if raw.LogBackend != "" {
		cfg.LogBackend = raw.LogBackend
	}
	if raw.TesseraStoragePath != "" {
		cfg.TesseraStoragePath = raw.TesseraStoragePath
	}
	if raw.TesseraLogName != "" {
		cfg.TesseraLogName = raw.TesseraLogName
	}
	if raw.SigningBackend != "" {
		cfg.SigningBackend = raw.SigningBackend
	}
	if raw.OIDCIssuer != "" {
		cfg.OIDCIssuer = raw.OIDCIssuer
	}
	if raw.OIDCClientID != "" {
		cfg.OIDCClientID = raw.OIDCClientID
	}
	if raw.FulcioURL != "" {
		cfg.FulcioURL = raw.FulcioURL
	}
	cfg.RekorAnchor = raw.RekorAnchor
	if raw.RekorURL != "" {
		cfg.RekorURL = raw.RekorURL
	}
	if raw.RekorAnchorInterval != "" {
		d, err := time.ParseDuration(raw.RekorAnchorInterval)
		if err != nil {
			return nil, fmt.Errorf("rekor_anchor_interval: %w", err)
		}
		cfg.RekorAnchorInterval = d
	}

	return &cfg, nil
}
