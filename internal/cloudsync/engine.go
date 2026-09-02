package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

type LocalStore interface {
	BackfillAllSyncMutations() error
	GetSyncState(targetKey string) (*store.SyncState, error)
	ListAllPendingSyncMutations(targetKey string, limit int) ([]store.SyncMutation, error)
	AckSyncMutationSeqs(targetKey string, seqs []int64) error
	ApplyPulledMutation(targetKey string, mutation store.SyncMutation) error
	RecordPulledSeq(targetKey string, seq int64) error
	MarkSyncHealthy(targetKey string) error
}

type Engine struct {
	local   LocalStore
	backend CloudBackend
	cfg     Config
}

func NewEngine(local LocalStore, backend CloudBackend, cfg Config) (*Engine, error) {
	validated, err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	return &Engine{local: local, backend: backend, cfg: validated}, nil
}

func (e *Engine) Sync(ctx context.Context) (*Result, error) {
	res := &Result{TargetKey: e.cfg.TargetKey}
	pushed, err := e.Push(ctx)
	if err != nil {
		return res, err
	}
	pulled, err := e.Pull(ctx)
	if err != nil {
		return res, err
	}
	res.Pushed = pushed.Pushed
	res.Pulled = pulled.Pulled
	res.SkippedOwn = pulled.SkippedOwn
	res.LatestSeq = pulled.LatestSeq
	state, err := e.local.GetSyncState(e.cfg.TargetKey)
	if err == nil {
		res.Lifecycle = state.Lifecycle
		pending, _ := e.local.ListAllPendingSyncMutations(e.cfg.TargetKey, 1_000_000)
		res.Pending = len(pending)
	}
	_ = e.local.MarkSyncHealthy(e.cfg.TargetKey)
	return res, nil
}

func (e *Engine) Push(ctx context.Context) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := e.local.BackfillAllSyncMutations(); err != nil {
		return nil, fmt.Errorf("backfill local sync mutations: %w", err)
	}
	res := &Result{TargetKey: e.cfg.TargetKey}
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		pending, err := e.local.ListAllPendingSyncMutations(e.cfg.TargetKey, e.cfg.BatchSize)
		if err != nil {
			return res, fmt.Errorf("list pending sync mutations: %w", err)
		}
		if len(pending) == 0 {
			break
		}
		entries := make([]MutationEntry, 0, len(pending))
		for _, mut := range pending {
			entries = append(entries, MutationEntry{
				LocalSeq: mut.Seq, Project: mut.Project, Entity: mut.Entity, EntityKey: mut.EntityKey,
				Op: mut.Op, Payload: json.RawMessage(mut.Payload), OccurredAt: mut.OccurredAt,
			})
		}
		push, err := e.backend.PushMutations(entries)
		if err != nil {
			return res, err
		}
		if push == nil || len(push.AcceptedSeqs) != len(entries) {
			return res, fmt.Errorf("cloud sync push acknowledged %d of %d mutations", len(push.AcceptedSeqs), len(entries))
		}
		if err := e.local.AckSyncMutationSeqs(e.cfg.TargetKey, push.AcceptedSeqs); err != nil {
			return res, fmt.Errorf("ack pushed mutations: %w", err)
		}
		res.Pushed += len(push.AcceptedSeqs)
	}
	state, err := e.local.GetSyncState(e.cfg.TargetKey)
	if err == nil {
		res.Lifecycle = state.Lifecycle
	}
	return res, nil
}

func (e *Engine) Pull(ctx context.Context) (*Result, error) {
	res := &Result{TargetKey: e.cfg.TargetKey}
	state, err := e.local.GetSyncState(e.cfg.TargetKey)
	if err != nil {
		return res, fmt.Errorf("get sync state: %w", err)
	}
	since := state.LastPulledSeq
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		batch, err := e.backend.PullMutations(since, e.cfg.BatchSize)
		if err != nil {
			return res, err
		}
		for _, remote := range batch.Mutations {
			if remote.Seq <= since {
				continue
			}
			if remote.OriginID == e.cfg.ClientID {
				if err := e.local.RecordPulledSeq(e.cfg.TargetKey, remote.Seq); err != nil {
					return res, fmt.Errorf("advance own remote seq %d: %w", remote.Seq, err)
				}
				res.SkippedOwn++
				since = remote.Seq
				continue
			}
			mutation := store.SyncMutation{
				Seq: remote.Seq, TargetKey: e.cfg.TargetKey, Entity: remote.Entity, EntityKey: remote.EntityKey,
				Op: remote.Op, Payload: string(remote.Payload), Source: store.SyncSourceRemote,
				Project: remote.Project, OccurredAt: remote.OccurredAt,
			}
			if err := e.local.ApplyPulledMutation(e.cfg.TargetKey, mutation); err != nil {
				return res, fmt.Errorf("apply pulled mutation seq=%d: %w", remote.Seq, err)
			}
			res.Pulled++
			since = remote.Seq
		}
		res.LatestSeq = since
		if !batch.HasMore {
			break
		}
	}
	state, err = e.local.GetSyncState(e.cfg.TargetKey)
	if err == nil {
		res.Lifecycle = state.Lifecycle
	}
	return res, nil
}
