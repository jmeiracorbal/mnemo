package mcp

import (
	"context"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/store"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

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

func TestHandleSyncStatusReadsStateAndPendingMutations(t *testing.T) {
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
	handler := handleSyncStatus(stub)
	result, err := handler(context.Background(), sdkmcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.IsError {
		t.Fatalf("unexpected sync status result: %#v", result)
	}
	if stub.stateTarget != store.DefaultSyncTargetKey || stub.listTarget != store.DefaultSyncTargetKey {
		t.Fatalf("status queried unexpected target: state=%q pending=%q", stub.stateTarget, stub.listTarget)
	}
	if stub.listLimit != 1_000_000 {
		t.Fatalf("status queried unexpected pending limit: %d", stub.listLimit)
	}
}
