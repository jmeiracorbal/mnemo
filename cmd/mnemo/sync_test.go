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

type syncStatusStoreStub struct {
	state       *store.SyncState
	pending     []store.SyncMutation
	stateTarget string
	listTarget  string
	listLimit   int
}

func (s *syncStatusStoreStub) GetSyncState(target string) (*store.SyncState, error) {
	s.stateTarget = target
	return s.state, nil
}

func (s *syncStatusStoreStub) ListAllPendingSyncMutations(target string, limit int) ([]store.SyncMutation, error) {
	s.listTarget = target
	s.listLimit = limit
	return s.pending, nil
}

func TestRunSyncStatusReadsStateAndPendingMutations(t *testing.T) {
	stub := &syncStatusStoreStub{
		state: &store.SyncState{
			TargetKey:       store.DefaultSyncTargetKey,
			Lifecycle:       store.SyncLifecyclePending,
			LastEnqueuedSeq: 12,
			LastAckedSeq:    9,
			LastPulledSeq:   7,
		},
		pending: []store.SyncMutation{{Seq: 10}, {Seq: 11}, {Seq: 12}},
	}
	err := runSyncStatus(stub, store.DefaultSyncTargetKey, true)
	if err != nil {
		t.Fatal(err)
	}
	if stub.stateTarget != store.DefaultSyncTargetKey || stub.listTarget != store.DefaultSyncTargetKey {
		t.Fatalf("status queried unexpected target: state=%q pending=%q", stub.stateTarget, stub.listTarget)
	}
	if stub.listLimit != 1_000_000 {
		t.Fatalf("status queried unexpected pending limit: %d", stub.listLimit)
	}
}
