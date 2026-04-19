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
	Addr                string        `json:"addr"`
	DB                  string        `json:"db"`
	BatchInterval       time.Duration `json:"batch_interval"`
	MaxBatch            int           `json:"max_batch"`
	Witnesses           string        `json:"witnesses"`
	Monitor             bool          `json:"monitor"`
	MonitorInterval     time.Duration `json:"monitor_interval"`
	DiscrepancyThreshold float64      `json:"discrepancy_threshold"`
	AlertWebhook        string        `json:"alert_webhook"`
	OperatorKey         string        `json:"operator_key"`
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

	return &cfg, nil
}
