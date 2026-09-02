package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func (s *Store) GetSyncState(targetKey string) (*SyncState, error) {
	targetKey = normalizeSyncTargetKey(targetKey)
	if err := s.ensureSyncState(targetKey); err != nil {
		return nil, err
	}
	return s.getSyncState(targetKey)
}

func (s *Store) ListPendingSyncMutations(targetKey string, limit int) ([]SyncMutation, error) {
	targetKey = normalizeSyncTargetKey(targetKey)
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.q.ListPendingSyncMutations(context.Background(), dbgen.ListPendingSyncMutationsParams{
		TargetKey: targetKey, ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	mutations := make([]SyncMutation, 0, len(rows))
	for _, row := range rows {
		mutations = append(mutations, SyncMutation{
			Seq: row.Seq, TargetKey: row.TargetKey, Entity: row.Entity, EntityKey: row.EntityKey,
			Op: row.Op, Payload: row.Payload, Source: row.Source, Project: row.Project,
			OccurredAt: row.OccurredAt, AckedAt: nullablePtr(row.AckedAt),
		})
	}
	return mutations, nil
}

func (s *Store) SkipAckNonEnrolledMutations(targetKey string) (int64, error) {
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.q.SkipNonEnrolledMutations(context.Background(), targetKey)
}

func (s *Store) AckSyncMutations(targetKey string, lastAckedSeq int64) error {
	if lastAckedSeq <= 0 {
		return nil
	}
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		state, err := s.getSyncStateTx(tx, targetKey)
		if err != nil {
			return err
		}
		if err := q.AckMutationsThrough(context.Background(), dbgen.AckMutationsThroughParams{
			TargetKey: targetKey, LastAckedSeq: lastAckedSeq,
		}); err != nil {
			return err
		}
		acked := state.LastAckedSeq
		if lastAckedSeq > acked {
			acked = lastAckedSeq
		}
		lifecycle := SyncLifecyclePending
		if acked >= state.LastEnqueuedSeq {
			lifecycle = SyncLifecycleHealthy
		}
		return q.UpdateSyncAckState(context.Background(), dbgen.UpdateSyncAckStateParams{
			LastAckedSeq: acked, Lifecycle: lifecycle, TargetKey: targetKey,
		})
	})
}

func (s *Store) AckSyncMutationSeqs(targetKey string, seqs []int64) error {
	if len(seqs) == 0 {
		return nil
	}
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		state, err := s.getSyncStateTx(tx, targetKey)
		if err != nil {
			return err
		}
		maxSeq := state.LastAckedSeq
		for _, seq := range seqs {
			if seq <= 0 {
				continue
			}
			if err := q.AckMutationSeq(context.Background(), dbgen.AckMutationSeqParams{
				TargetKey: targetKey, Seq: seq,
			}); err != nil {
				return err
			}
			if seq > maxSeq {
				maxSeq = seq
			}
		}
		remaining, err := q.CountPendingMutations(context.Background(), targetKey)
		if err != nil {
			return err
		}
		lifecycle := SyncLifecyclePending
		if remaining == 0 {
			lifecycle = SyncLifecycleHealthy
		}
		return q.UpdateSyncAckState(context.Background(), dbgen.UpdateSyncAckStateParams{
			LastAckedSeq: maxSeq, Lifecycle: lifecycle, TargetKey: targetKey,
		})
	})
}

func (s *Store) AcquireSyncLease(targetKey, owner string, ttl time.Duration, now time.Time) (bool, error) {
	targetKey = normalizeSyncTargetKey(targetKey)
	if ttl <= 0 {
		ttl = time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if err := s.ensureSyncState(targetKey); err != nil {
		return false, err
	}
	rows, err := s.q.AcquireSyncLease(context.Background(), dbgen.AcquireSyncLeaseParams{
		Owner: sqlNullString(owner), LeaseUntil: sqlNullString(now.Add(ttl).UTC().Format(time.RFC3339)),
		Lifecycle: SyncLifecycleRunning, TargetKey: targetKey, Now: now.UTC().Format(time.RFC3339),
	})
	return rows > 0, err
}

func (s *Store) ReleaseSyncLease(targetKey, owner string) error {
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.q.ReleaseSyncLease(context.Background(), dbgen.ReleaseSyncLeaseParams{
		TargetKey: targetKey, Owner: sqlNullString(owner),
	})
}

func (s *Store) MarkSyncFailure(targetKey, message string, backoffUntil time.Time) error {
	targetKey = normalizeSyncTargetKey(targetKey)
	backoff := backoffUntil.UTC().Format(time.RFC3339)
	if err := s.ensureSyncState(targetKey); err != nil {
		return err
	}
	return s.q.MarkSyncFailure(context.Background(), dbgen.MarkSyncFailureParams{
		Lifecycle: SyncLifecycleDegraded, BackoffUntil: sqlNullString(backoff),
		LastError: sqlNullString(message), TargetKey: targetKey,
	})
}

func (s *Store) MarkSyncHealthy(targetKey string) error {
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.q.MarkSyncHealthy(context.Background(), dbgen.MarkSyncHealthyParams{
		Lifecycle: SyncLifecycleHealthy, TargetKey: targetKey,
	})
}

func (s *Store) ApplyPulledMutation(targetKey string, mutation SyncMutation) error {
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.withTx(func(tx *sql.Tx) error {
		state, err := s.getSyncStateTx(tx, targetKey)
		if err != nil {
			return err
		}
		if mutation.Seq <= state.LastPulledSeq {
			return nil
		}
		switch mutation.Entity {
		case SyncEntitySession:
			if mutation.Op != SyncOpUpsert {
				return fmt.Errorf("sync: unsupported op %q for %q", mutation.Op, mutation.Entity)
			}
			var payload syncSessionPayload
			if err := decodeSyncPayload([]byte(mutation.Payload), &payload); err != nil {
				return err
			}
			if err := s.applySessionPayloadTx(tx, payload); err != nil {
				return err
			}
		case SyncEntityObservation:
			var payload syncObservationPayload
			if err := decodeSyncPayload([]byte(mutation.Payload), &payload); err != nil {
				return err
			}
			if mutation.Op == SyncOpDelete {
				if err := s.applyObservationDeleteTx(tx, payload); err != nil {
					return err
				}
			} else {
				if err := s.applyObservationUpsertTx(tx, payload); err != nil {
					return err
				}
			}
		case SyncEntityPrompt:
			if mutation.Op != SyncOpUpsert {
				return fmt.Errorf("sync: unsupported op %q for %q", mutation.Op, mutation.Entity)
			}
			var payload syncPromptPayload
			if err := decodeSyncPayload([]byte(mutation.Payload), &payload); err != nil {
				return err
			}
			if err := s.applyPromptUpsertTx(tx, payload); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown sync entity %q", mutation.Entity)
		}

		if mutation.Source == SyncSourceRemote {
			if err := s.recordAppliedRemoteMutationTx(tx, targetKey, mutation); err != nil {
				return err
			}
		}

		return s.q.WithTx(tx).UpdateLastPulledSeq(context.Background(), dbgen.UpdateLastPulledSeqParams{
			LastPulledSeq: mutation.Seq, Lifecycle: SyncLifecycleHealthy, TargetKey: targetKey,
		})
	})
}

func (s *Store) recordAppliedRemoteMutationTx(tx *sql.Tx, targetKey string, mutation SyncMutation) error {
	project := mutation.Project
	if project == "" {
		project = extractProjectFromRawPayload(mutation.Payload)
	}
	res, err := s.execHook(tx, `
		INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source, project, occurred_at, acked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		targetKey, mutation.Entity, mutation.EntityKey, mutation.Op, mutation.Payload, SyncSourceLocal, project)
	if err != nil {
		return err
	}
	localSeq, err := res.LastInsertId()
	if err != nil {
		return err
	}
	_, err = s.execHook(tx, `
		UPDATE sync_state
		SET last_enqueued_seq = CASE WHEN last_enqueued_seq < ? THEN ? ELSE last_enqueued_seq END,
		    last_acked_seq = CASE WHEN last_acked_seq < ? THEN ? ELSE last_acked_seq END,
		    lifecycle = ?,
		    updated_at = datetime('now')
		WHERE target_key = ?`, localSeq, localSeq, localSeq, localSeq, SyncLifecycleHealthy, targetKey)
	return err
}

func extractProjectFromRawPayload(payload string) string {
	var raw struct {
		Project *string `json:"project"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil || raw.Project == nil {
		return ""
	}
	return strings.TrimSpace(*raw.Project)
}

func (s *Store) ensureSyncState(targetKey string) error {
	return s.q.EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: targetKey, Lifecycle: SyncLifecycleIdle,
	})
}

func (s *Store) getSyncState(targetKey string) (*SyncState, error) {
	row, err := s.q.GetSyncState(context.Background(), targetKey)
	if err != nil {
		return nil, err
	}
	return syncStateFromDB(row), nil
}

func (s *Store) getSyncStateTx(tx *sql.Tx, targetKey string) (*SyncState, error) {
	q := s.q.WithTx(tx)
	if err := q.EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: targetKey, Lifecycle: SyncLifecycleIdle,
	}); err != nil {
		return nil, err
	}
	row, err := q.GetSyncState(context.Background(), targetKey)
	if err != nil {
		return nil, err
	}
	return syncStateFromDB(row), nil
}

func (s *Store) backfillProjectSyncMutationsTx(tx *sql.Tx, project string) error {
	if err := s.backfillSessionSyncMutationsTx(tx, project); err != nil {
		return err
	}
	if err := s.backfillObservationSyncMutationsTx(tx, project); err != nil {
		return err
	}
	return s.backfillPromptSyncMutationsTx(tx, project)
}

func (s *Store) repairEnrolledProjectSyncMutations() error {
	return s.withTx(func(tx *sql.Tx) error {
		rows, err := s.q.WithTx(tx).ListEnrolledProjects(context.Background())
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := s.backfillProjectSyncMutationsTx(tx, row.Project); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) backfillSessionSyncMutationsTx(tx *sql.Tx, project string) error {
	q := s.q.WithTx(tx)
	rows, err := q.ListSessionsMissingSyncMutation(context.Background(), dbgen.ListSessionsMissingSyncMutationParams{
		ProjectName: project, TargetKey: DefaultSyncTargetKey, Entity: SyncEntitySession, Source: SyncSourceLocal,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		payload := syncSessionPayload{
			ID: row.ID, Project: row.Project, Directory: row.Directory,
			EndedAt: nullablePtr(row.EndedAt), Summary: nullablePtr(row.Summary),
			Provenance: provenanceInputForID(q, row.ProvenanceID),
		}
		var sess Session
		sess.ID = payload.ID
		if err := s.loadTagsForSessionTx(tx, &sess); err != nil {
			return fmt.Errorf("backfill session tags: %w", err)
		}
		tags := sess.Tags
		if tags == nil {
			tags = []string{}
		}
		payload.Tags = &tags
		if err := s.enqueueSyncMutationTx(tx, SyncEntitySession, payload.ID, SyncOpUpsert, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillObservationSyncMutationsTx(tx *sql.Tx, project string) error {
	q := s.q.WithTx(tx)
	rows, err := q.ListObservationsMissingSyncMutation(context.Background(), dbgen.ListObservationsMissingSyncMutationParams{
		ProjectName: sqlNullString(project), TargetKey: DefaultSyncTargetKey,
		Entity: SyncEntityObservation, Source: SyncSourceLocal,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		obsID := row.ID
		payload := syncObservationPayload{
			SyncID: dbString(row.SyncID), SessionID: row.SessionID, Type: row.Type,
			Title: row.Title, Content: row.Content, ToolName: nullablePtr(row.ToolName),
			Project: nullablePtr(row.Project), Scope: row.Scope, TopicKey: nullablePtr(row.TopicKey),
			Provenance: provenanceInputForID(q, row.ProvenanceID),
		}
		var o Observation
		o.ID = obsID
		if err := s.loadTagsForObservationTx(tx, &o); err != nil {
			return fmt.Errorf("backfill observation tags: %w", err)
		}
		tags := o.Tags
		if tags == nil {
			tags = []string{}
		}
		payload.Tags = &tags
		if err := s.enqueueSyncMutationTx(tx, SyncEntityObservation, payload.SyncID, SyncOpUpsert, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillPromptSyncMutationsTx(tx *sql.Tx, project string) error {
	q := s.q.WithTx(tx)
	rows, err := q.ListPromptsMissingSyncMutation(context.Background(), dbgen.ListPromptsMissingSyncMutationParams{
		ProjectName: sqlNullString(project), TargetKey: DefaultSyncTargetKey,
		Entity: SyncEntityPrompt, Source: SyncSourceLocal,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		proj := nullablePtr(row.Project)
		payload := syncPromptPayload{
			SyncID: dbString(row.SyncID), SessionID: row.SessionID, Content: row.Content, Project: proj,
			Provenance: provenanceInputForID(q, row.ProvenanceID),
		}
		if err := s.enqueueSyncMutationTx(tx, SyncEntityPrompt, payload.SyncID, SyncOpUpsert, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) enqueueSyncMutationTx(tx *sql.Tx, entity, entityKey, op string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	project := extractProjectFromPayload(payload)
	q := s.q.WithTx(tx)
	if err := q.EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: DefaultSyncTargetKey, Lifecycle: SyncLifecycleIdle,
	}); err != nil {
		return err
	}
	seq, err := q.InsertSyncMutation(context.Background(), dbgen.InsertSyncMutationParams{
		TargetKey: DefaultSyncTargetKey, Entity: entity, EntityKey: entityKey, Op: op,
		Payload: string(encoded), Source: SyncSourceLocal, Project: project,
	})
	if err != nil {
		return err
	}
	return q.UpdateLastEnqueuedSeq(context.Background(), dbgen.UpdateLastEnqueuedSeqParams{
		Lifecycle: SyncLifecyclePending, LastEnqueuedSeq: seq, TargetKey: DefaultSyncTargetKey,
	})
}

func (s *Store) applySessionPayloadTx(tx *sql.Tx, payload syncSessionPayload) error {
	q := s.q.WithTx(tx)
	provenanceInput := provenanceInputFromPtr(payload.Provenance)
	if !hasProvenanceInput(provenanceInput) {
		if _, err := q.GetSessionPayload(context.Background(), payload.ID); err == sql.ErrNoRows {
			provenanceInput = SyncPullProvenance()
		} else if err != nil {
			return err
		}
	}
	provenanceID, err := s.optionalProvenanceTx(tx, provenanceInput)
	if err != nil {
		return err
	}
	if err := q.ApplySessionPayload(context.Background(), dbgen.ApplySessionPayloadParams{
		ID: payload.ID, Project: payload.Project, Directory: payload.Directory,
		EndedAt: sqlNullStringPtr(payload.EndedAt), Summary: sqlNullStringPtr(payload.Summary),
		ProvenanceID: provenanceID,
	}); err != nil {
		return err
	}
	if payload.Tags != nil {
		if err := s.setTagsForSessionTx(tx, payload.ID, *payload.Tags); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyObservationUpsertTx(tx *sql.Tx, payload syncObservationPayload) error {
	q := s.q.WithTx(tx)
	existing, err := s.getObservationBySyncIDTx(tx, payload.SyncID, true)
	if err == sql.ErrNoRows {
		provenanceInput := provenanceInputFromPtr(payload.Provenance)
		if !hasProvenanceInput(provenanceInput) {
			provenanceInput = SyncPullProvenance()
		}
		provenanceID, err := s.optionalProvenanceTx(tx, provenanceInput)
		if err != nil {
			return err
		}
		newID, err := q.InsertPulledObservation(context.Background(), dbgen.InsertPulledObservationParams{
			SyncID: sqlNullString(payload.SyncID), SessionID: payload.SessionID, Type: payload.Type,
			Title: payload.Title, Content: payload.Content, ToolName: sqlNullStringPtr(payload.ToolName),
			Project: sqlNullStringPtr(payload.Project), Scope: normalizeScope(payload.Scope),
			TopicKey: sqlNullStringPtr(payload.TopicKey), NormalizedHash: sqlNullString(hashNormalized(payload.Content)),
			ProvenanceID: provenanceID,
		})
		if err != nil {
			return err
		}
		if payload.Tags != nil {
			if err := s.setTagsForObservationTx(tx, newID, *payload.Tags); err != nil {
				return err
			}
		}
		return nil
	}
	if err != nil {
		return err
	}
	provenanceID, err := s.optionalProvenanceTx(tx, provenanceInputFromPtr(payload.Provenance))
	if err != nil {
		return err
	}
	err = q.UpdatePulledObservation(context.Background(), dbgen.UpdatePulledObservationParams{
		SessionID: payload.SessionID, Type: payload.Type, Title: payload.Title, Content: payload.Content,
		ToolName: sqlNullStringPtr(payload.ToolName), Project: sqlNullStringPtr(payload.Project),
		Scope: normalizeScope(payload.Scope), TopicKey: sqlNullStringPtr(payload.TopicKey),
		NormalizedHash: sqlNullString(hashNormalized(payload.Content)), ProvenanceID: provenanceID, ID: existing.ID,
	})
	if err != nil {
		return err
	}
	if payload.Tags != nil {
		if err := s.setTagsForObservationTx(tx, existing.ID, *payload.Tags); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyObservationDeleteTx(tx *sql.Tx, payload syncObservationPayload) error {
	existing, err := s.getObservationBySyncIDTx(tx, payload.SyncID, true)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if payload.HardDelete {
		return s.q.WithTx(tx).DeleteObservationByID(context.Background(), existing.ID)
	}
	deletedAt := payload.DeletedAt
	if deletedAt == nil {
		now := Now()
		deletedAt = &now
	}
	return s.q.WithTx(tx).SetObservationDeletedAt(context.Background(), dbgen.SetObservationDeletedAtParams{
		DeletedAt: sqlNullStringPtr(deletedAt), ID: existing.ID,
	})
}

func (s *Store) applyPromptUpsertTx(tx *sql.Tx, payload syncPromptPayload) error {
	q := s.q.WithTx(tx)
	existingID, err := q.FindPromptBySyncID(context.Background(), sqlNullString(payload.SyncID))
	if err == sql.ErrNoRows {
		provenanceInput := provenanceInputFromPtr(payload.Provenance)
		if !hasProvenanceInput(provenanceInput) {
			provenanceInput = SyncPullProvenance()
		}
		provenanceID, err := s.optionalProvenanceTx(tx, provenanceInput)
		if err != nil {
			return err
		}
		_, err = q.InsertPrompt(context.Background(), dbgen.InsertPromptParams{
			SyncID: sqlNullString(payload.SyncID), SessionID: payload.SessionID,
			Content: payload.Content, Project: sqlNullStringPtr(payload.Project), ProvenanceID: provenanceID,
		})
		return err
	}
	if err != nil {
		return err
	}
	provenanceID, err := s.optionalProvenanceTx(tx, provenanceInputFromPtr(payload.Provenance))
	if err != nil {
		return err
	}
	return q.UpdatePrompt(context.Background(), dbgen.UpdatePromptParams{
		SessionID: payload.SessionID, Content: payload.Content,
		Project: sqlNullStringPtr(payload.Project), ProvenanceID: provenanceID, ID: existingID,
	})
}

// BackfillAllSyncMutations ensures all locally stored sessions, observations, and prompts
// have pending sync mutations. Cloud sync is complete by default; enrollment is kept only
// for backward-compatible legacy commands and is not used by the cloud sync engine.
func (s *Store) BackfillAllSyncMutations() error {
	return s.withTx(func(tx *sql.Tx) error {
		rows, err := s.queryItHook(tx, `
			SELECT DISTINCT project FROM (
				SELECT project FROM sessions
				UNION SELECT ifnull(project, '') AS project FROM observations
				UNION SELECT ifnull(project, '') AS project FROM user_prompts
			) ORDER BY project ASC`)
		if err != nil {
			return err
		}
		defer rows.Close()

		projects := make([]string, 0)
		for rows.Next() {
			var project string
			if err := rows.Scan(&project); err != nil {
				return err
			}
			projects = append(projects, project)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(projects) == 0 {
			if err := s.ensureSyncStateTx(tx, DefaultSyncTargetKey); err != nil {
				return err
			}
		}
		for _, project := range projects {
			if err := s.backfillProjectSyncMutationsTx(tx, project); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListAllPendingSyncMutations returns all pending local mutations regardless of
// legacy project enrollment. Cloud sync semantics require every local mutation to
// be uploaded idempotently.
func (s *Store) ListAllPendingSyncMutations(targetKey string, limit int) ([]SyncMutation, error) {
	targetKey = normalizeSyncTargetKey(targetKey)
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.queryItHook(s.db, `
		SELECT seq, target_key, entity, entity_key, op, payload, source, project, occurred_at, acked_at
		FROM sync_mutations
		WHERE target_key = ? AND acked_at IS NULL
		ORDER BY seq ASC
		LIMIT ?`, targetKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mutations := make([]SyncMutation, 0)
	for rows.Next() {
		var mutation SyncMutation
		if err := rows.Scan(&mutation.Seq, &mutation.TargetKey, &mutation.Entity, &mutation.EntityKey, &mutation.Op, &mutation.Payload, &mutation.Source, &mutation.Project, &mutation.OccurredAt, &mutation.AckedAt); err != nil {
			return nil, err
		}
		mutations = append(mutations, mutation)
	}
	return mutations, rows.Err()
}

// RecordPulledSeq advances the pull cursor without applying a payload. It is used
// for idempotently acknowledging this client's own mutations after they round-trip
// through the cloud journal.
func (s *Store) RecordPulledSeq(targetKey string, seq int64) error {
	if seq <= 0 {
		return nil
	}
	targetKey = normalizeSyncTargetKey(targetKey)
	return s.withTx(func(tx *sql.Tx) error {
		state, err := s.getSyncStateTx(tx, targetKey)
		if err != nil {
			return err
		}
		if seq <= state.LastPulledSeq {
			return nil
		}
		return s.q.WithTx(tx).UpdateLastPulledSeq(context.Background(), dbgen.UpdateLastPulledSeqParams{
			LastPulledSeq: seq, Lifecycle: SyncLifecycleHealthy, TargetKey: targetKey,
		})
	})
}

func (s *Store) ensureSyncStateTx(tx *sql.Tx, targetKey string) error {
	return s.q.WithTx(tx).EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: normalizeSyncTargetKey(targetKey), Lifecycle: SyncLifecycleIdle,
	})
}
