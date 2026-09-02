package main

import (
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

func TestResolveCloudConfigFailsWithoutCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MNEMO_CLOUD_URL", "")
	t.Setenv("MNEMO_CLOUD_KEY", "")
	t.Setenv("MNEMO_CLOUD_CLIENT_ID", "")
	_, err := resolveCloudConfig(syncCloudFlags{})
	if err == nil || !strings.Contains(err.Error(), "MNEMO_CLOUD_URL is required") {
		t.Fatalf("expected missing URL config error, got %v", err)
	}
}

func TestResolveCloudConfigUsesDefaultTarget(t *testing.T) {
	t.Setenv("MNEMO_CLOUD_URL", "")
	t.Setenv("MNEMO_CLOUD_KEY", "")
	t.Setenv("MNEMO_CLOUD_CLIENT_ID", "")
	cfg, err := resolveCloudConfig(syncCloudFlags{
		url:      "https://example.turso.io",
		key:      "tok",
		clientID: "client-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TargetKey != store.DefaultSyncTargetKey {
		t.Fatalf("expected default target %q, got %q", store.DefaultSyncTargetKey, cfg.TargetKey)
	}
}

func TestResolveCloudConfigFlagsTakePrecedenceOverEnv(t *testing.T) {
	t.Setenv("MNEMO_CLOUD_URL", "https://env.example.com")
	t.Setenv("MNEMO_CLOUD_KEY", "env-key")
	t.Setenv("MNEMO_CLOUD_CLIENT_ID", "env-client")
	cfg, err := resolveCloudConfig(syncCloudFlags{
		url:      "libsql://flag.turso.io",
		key:      "flag-key",
		clientID: "flag-client",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "libsql://flag.turso.io" {
		t.Fatalf("expected flag URL, got %q", cfg.URL)
	}
	if cfg.Key != "flag-key" {
		t.Fatalf("expected flag key, got %q", cfg.Key)
	}
}
