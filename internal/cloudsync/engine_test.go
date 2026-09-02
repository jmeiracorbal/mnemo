package cloudsync

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

type fakeStore struct {
	state      store.SyncState
	pending    []store.SyncMutation
	acked      []int64
	applied    []store.SyncMutation
	advanced   []int64
	backfilled int
}

func (f *fakeStore) BackfillAllSyncMutations() error { f.backfilled++; return nil }
func (f *fakeStore) GetSyncState(targetKey string) (*store.SyncState, error) {
	st := f.state
	st.TargetKey = targetKey
	return &st, nil
}
func (f *fakeStore) ListAllPendingSyncMutations(targetKey string, limit int) ([]store.SyncMutation, error) {
	if len(f.pending) == 0 {
		return nil, nil
	}
	if limit > 0 && limit < len(f.pending) {
		out := append([]store.SyncMutation(nil), f.pending[:limit]...)
		f.pending = f.pending[limit:]
		return out, nil
	}
	out := append([]store.SyncMutation(nil), f.pending...)
	f.pending = nil
	return out, nil
}
func (f *fakeStore) AckSyncMutationSeqs(targetKey string, seqs []int64) error {
	f.acked = append(f.acked, seqs...)
	return nil
}
func (f *fakeStore) ApplyPulledMutation(targetKey string, mutation store.SyncMutation) error {
	f.applied = append(f.applied, mutation)
	f.state.LastPulledSeq = mutation.Seq
	return nil
}
func (f *fakeStore) RecordPulledSeq(targetKey string, seq int64) error {
	f.advanced = append(f.advanced, seq)
	f.state.LastPulledSeq = seq
	return nil
}
func (f *fakeStore) MarkSyncHealthy(targetKey string) error {
	f.state.Lifecycle = store.SyncLifecycleHealthy
	return nil
}

type fakeBackend struct {
	pushed []MutationEntry
	pulls  []*PullResult
}

func (f *fakeBackend) PushMutations(entries []MutationEntry) (*PushResult, error) {
	f.pushed = append(f.pushed, entries...)
	seqs := make([]int64, len(entries))
	for i, e := range entries {
		seqs[i] = e.LocalSeq
	}
	return &PushResult{AcceptedSeqs: seqs}, nil
}
func (f *fakeBackend) PullMutations(sinceSeq int64, limit int) (*PullResult, error) {
	if len(f.pulls) == 0 {
		return &PullResult{}, nil
	}
	res := f.pulls[0]
	f.pulls = f.pulls[1:]
	return res, nil
}

func TestEnginePushBackfillsAndAcksAcceptedSeqs(t *testing.T) {
	local := &fakeStore{pending: []store.SyncMutation{{Seq: 1, Entity: store.SyncEntitySession, EntityKey: "s1", Op: store.SyncOpUpsert, Payload: `{"id":"s1"}`}, {Seq: 2, Entity: store.SyncEntityObservation, EntityKey: "o1", Op: store.SyncOpUpsert, Payload: `{"sync_id":"o1"}`}}}
	backend := &fakeBackend{}
	engine, err := NewEngine(local, backend, Config{URL: "libsql://example.turso.io", Key: "publishable", ClientID: "client-a", TargetKey: store.DefaultSyncTargetKey, BatchSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.Push(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Pushed != 2 || local.backfilled != 1 {
		t.Fatalf("unexpected result/backfill: %#v backfilled=%d", res, local.backfilled)
	}
	if !reflect.DeepEqual(local.acked, []int64{1, 2}) {
		t.Fatalf("acked seqs = %#v", local.acked)
	}
}

func TestEnginePullSkipsOwnRowsAndAppliesForeignRows(t *testing.T) {
	payload := json.RawMessage(`{"id":"s2","project":"brain","directory":"/tmp"}`)
	local := &fakeStore{}
	backend := &fakeBackend{pulls: []*PullResult{{Mutations: []PulledMutation{{Seq: 10, OriginID: "client-a", ClientSeq: 1, Entity: store.SyncEntitySession, EntityKey: "s1", Op: store.SyncOpUpsert, Payload: payload}, {Seq: 11, OriginID: "client-b", ClientSeq: 7, Project: "brain", Entity: store.SyncEntitySession, EntityKey: "s2", Op: store.SyncOpUpsert, Payload: payload}}}}}
	engine, err := NewEngine(local, backend, Config{URL: "libsql://example.turso.io", Key: "publishable", ClientID: "client-a", TargetKey: store.DefaultSyncTargetKey})
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.Pull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.SkippedOwn != 1 || res.Pulled != 1 || res.LatestSeq != 11 {
		t.Fatalf("unexpected pull result: %#v", res)
	}
	if !reflect.DeepEqual(local.advanced, []int64{10}) {
		t.Fatalf("advanced seqs = %#v", local.advanced)
	}
	if len(local.applied) != 1 || local.applied[0].Seq != 11 || local.applied[0].Project != "brain" {
		t.Fatalf("applied = %#v", local.applied)
	}
}

func TestEnginePushRejectsPartialAck(t *testing.T) {
	local := &fakeStore{pending: []store.SyncMutation{{Seq: 1, Entity: store.SyncEntitySession, EntityKey: "s1", Op: store.SyncOpUpsert, Payload: `{"id":"s1"}`}, {Seq: 2, Entity: store.SyncEntitySession, EntityKey: "s2", Op: store.SyncOpUpsert, Payload: `{"id":"s2"}`}}}
	backend := &partialAckBackend{}
	engine, err := NewEngine(local, backend, Config{URL: "libsql://example.turso.io", Key: "publishable", ClientID: "client-a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Push(context.Background()); err == nil {
		t.Fatal("expected partial ack error")
	}
	if len(local.acked) != 0 {
		t.Fatalf("partial ack must not ack local rows: %#v", local.acked)
	}
}

type partialAckBackend struct{}

func (partialAckBackend) PushMutations(entries []MutationEntry) (*PushResult, error) {
	return &PushResult{AcceptedSeqs: []int64{entries[0].LocalSeq}}, nil
}
func (partialAckBackend) PullMutations(sinceSeq int64, limit int) (*PullResult, error) {
	return &PullResult{}, nil
}

func TestEnginePullPaginates(t *testing.T) {
	local := &fakeStore{}
	payload := json.RawMessage(`{"id":"s","project":"brain","directory":"/tmp"}`)
	backend := &fakeBackend{pulls: []*PullResult{
		{Mutations: []PulledMutation{{Seq: 1, OriginID: "client-b", Entity: store.SyncEntitySession, EntityKey: "s1", Op: store.SyncOpUpsert, Payload: payload}}, HasMore: true},
		{Mutations: []PulledMutation{{Seq: 2, OriginID: "client-b", Entity: store.SyncEntitySession, EntityKey: "s2", Op: store.SyncOpUpsert, Payload: payload}}, HasMore: false},
	}}
	engine, err := NewEngine(local, backend, Config{URL: "libsql://example.turso.io", Key: "publishable", ClientID: "client-a", BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.Pull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Pulled != 2 || res.LatestSeq != 2 || len(local.applied) != 2 {
		t.Fatalf("unexpected pagination result=%#v applied=%#v", res, local.applied)
	}
}
