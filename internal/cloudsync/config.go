package cloudsync

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

// Config holds the credentials and tuning parameters for cloud sync.
// Provider is always an SQLite-compatible cloud database (e.g. "turso").
type Config struct {
	Provider  string
	URL       string
	Key       string
	ClientID  string
	TargetKey string
	BatchSize int
	Timeout   time.Duration
}

// ConfigFromEnv builds a Config by layering sources (lowest → highest priority):
//  1. File config (~/.config/mnemo/cloud.toml)
//  2. Environment variables (MNEMO_CLOUD_*)
//
// CLI flags are applied by the caller on top of the returned Config.
// Returns an error only if the final config fails Validate — callers that
// merely want to check whether cloud is configured should call LoadFileConfig
// directly.
func ConfigFromEnv() (Config, error) {
	// Start from file config (lowest priority).
	cfg, _ := LoadFileConfig()

	// Override with env vars.
	if v := os.Getenv("MNEMO_CLOUD_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("MNEMO_CLOUD_KEY"); v != "" {
		cfg.Key = v
	}
	if v := os.Getenv("MNEMO_CLOUD_CLIENT_ID"); v != "" {
		cfg.ClientID = v
	}
	if v := os.Getenv("MNEMO_CLOUD_TARGET"); v != "" {
		cfg.TargetKey = v
	}

	// Apply defaults.
	if cfg.Provider == "" {
		cfg.Provider = "turso"
	}
	if cfg.TargetKey == "" {
		cfg.TargetKey = store.DefaultSyncTargetKey
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	return cfg.Validate()
}

// NewBackend creates the CloudBackend for cfg by delegating to the registered
// provider. For Turso this also runs idempotent migrations.
func NewBackend(cfg Config) (CloudBackend, error) {
	p, err := LookupProvider(cfg.Provider)
	if err != nil {
		return nil, err
	}
	return p.NewBackend(cfg)
}

// Ping tests the cloud connection without creating a full backend.
func Ping(cfg Config) error {
	p, err := LookupProvider(cfg.Provider)
	if err != nil {
		return err
	}
	return p.Ping(cfg)
}

func (c Config) Validate() (Config, error) {
	c.URL = strings.TrimRight(strings.TrimSpace(c.URL), "/")
	c.Key = strings.TrimSpace(c.Key)
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.TargetKey = strings.TrimSpace(c.TargetKey)
	if c.Provider == "" {
		c.Provider = "turso"
	}
	if c.TargetKey == "" {
		c.TargetKey = store.DefaultSyncTargetKey
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 25
	}
	if c.Timeout <= 0 {
		c.Timeout = 60 * time.Second
	}
	if c.URL == "" {
		return c, fmt.Errorf("MNEMO_CLOUD_URL is required")
	}
	parsed, err := url.Parse(c.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https" &&
			parsed.Scheme != "libsql" && parsed.Scheme != "libsqls") {
		return c, fmt.Errorf("MNEMO_CLOUD_URL must be a valid URL (https:// or libsql://)")
	}
	if c.Key == "" {
		return c, fmt.Errorf("MNEMO_CLOUD_KEY is required")
	}
	if c.ClientID == "" {
		return c, fmt.Errorf("MNEMO_CLOUD_CLIENT_ID is required")
	}
	return c, nil
}
