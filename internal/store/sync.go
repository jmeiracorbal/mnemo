package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
			if mutation.Op != SyncOpUpsert {
				return fmt.Errorf("sync: unsupported op %q for %q", mutation.Op, mutation.Entity)
			}
			var payload syncObservationPayload
			if err := decodeSyncPayload([]byte(mutation.Payload), &payload); err != nil {
				return err
			}
			if err := s.applyObservationUpsertTx(tx, payload); err != nil {
				return err
			}
		case SyncEntityUserPrompt:
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
		case SyncEntityProject, SyncEntityAgent, SyncEntityTool, SyncEntityModel, SyncEntitySourceKind, SyncEntityMCPClient, SyncEntityProvenanceContext, SyncEntityObservationTag, SyncEntitySessionTag, SyncEntityObservationReview:
			if mutation.Op != SyncOpUpsert {
				return fmt.Errorf("sync: unsupported op %q for %q", mutation.Op, mutation.Entity)
			}
			if err := s.applyCanonicalPayloadTx(tx, mutation.Entity, mutation.Payload); err != nil {
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

func (s *Store) applyCanonicalPayloadTx(tx *sql.Tx, entity, raw string) error {
	var p map[string]any
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	str := func(k string) string {
		if v, ok := p[k].(string); ok {
			return v
		}
		return ""
	}
	deleted := 0
	if v, ok := p["is_deleted"].(bool); ok && v {
		deleted = 1
	}
	switch entity {
	case SyncEntityProject:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO projects(id,name,is_deleted) VALUES (?,?,?)`, str("id"), str("name"), deleted)
		return err
	case SyncEntityAgent:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO agents(id,display_name,kind,is_deleted) VALUES (?,?,?,?)`, str("id"), str("display_name"), str("kind"), deleted)
		return err
	case SyncEntityTool:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO tools(id,display_name,is_deleted) VALUES (?,?,?)`, str("id"), str("display_name"), deleted)
		return err
	case SyncEntityModel:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO models(id,provider,display_name,is_deleted) VALUES (?,?,?,?)`, str("id"), str("provider"), str("display_name"), deleted)
		return err
	case SyncEntitySourceKind:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO source_kinds(id,display_name,is_deleted) VALUES (?,?,?)`, str("id"), str("display_name"), deleted)
		return err
	case SyncEntityMCPClient:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO mcp_clients(id,name,version,transport,is_deleted) VALUES (?,?,?,?,?)`, str("id"), str("name"), str("version"), str("transport"), deleted)
		return err
	case SyncEntityProvenanceContext:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO provenance_contexts(id,agent_id,source_kind_id,tool_id,model_id,mcp_client_id,is_deleted) VALUES (?,?,?,?,?,?,?)`, int64(numberValue(p["id"])), str("agent_id"), str("source_kind_id"), str("tool_id"), str("model_id"), str("mcp_client_id"), deleted)
		return err
	case SyncEntityObservationTag:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO observation_tags(observation_id,tag,is_deleted) SELECT id,?,? FROM observations WHERE sync_id=?`, str("tag"), deleted, str("observation_sync_id"))
		return err
	case SyncEntitySessionTag:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO session_tags(session_id,tag,is_deleted) VALUES (?,?,?)`, str("session_id"), str("tag"), deleted)
		return err
	case SyncEntityObservationReview:
		_, err := s.execHook(tx, `INSERT OR REPLACE INTO observation_reviews(observation_id,state,reason,reviewed_at,is_deleted) SELECT id,?,?,?,? FROM observations WHERE sync_id=?`, str("state"), str("reason"), str("reviewed_at"), deleted, str("observation_sync_id"))
		return err
	}
	return fmt.Errorf("unknown canonical entity %q", entity)
}

func numberValue(v any) int64 {
	if n, ok := v.(float64); ok {
		return int64(n)
	}
	return 0
}

func (s *Store) recordAppliedRemoteMutationTx(tx *sql.Tx, targetKey string, mutation SyncMutation) error {
	res, err := s.execHook(tx, `
		INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source, occurred_at, acked_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		targetKey, mutation.Entity, mutation.EntityKey, mutation.Op, mutation.Payload, SyncSourceLocal)
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

func (s *Store) ensureSyncState(targetKey string) error {
	if err := s.q.EnsureSyncType(context.Background(), dbgen.EnsureSyncTypeParams{
		ID: DefaultSyncTypeID, DisplayName: "Cloud synchronization",
	}); err != nil {
		return err
	}
	return s.q.EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: targetKey, SyncTypeID: DefaultSyncTypeID, Lifecycle: SyncLifecycleIdle,
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
	if err := q.EnsureSyncType(context.Background(), dbgen.EnsureSyncTypeParams{
		ID: DefaultSyncTypeID, DisplayName: "Cloud synchronization",
	}); err != nil {
		return nil, err
	}
	if err := q.EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: targetKey, SyncTypeID: DefaultSyncTypeID, Lifecycle: SyncLifecycleIdle,
	}); err != nil {
		return nil, err
	}
	row, err := q.GetSyncState(context.Background(), targetKey)
	if err != nil {
		return nil, err
	}
	return syncStateFromDB(row), nil
}

func (s *Store) enqueueSyncMutationTx(tx *sql.Tx, entity, entityKey, op string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	q := s.q.WithTx(tx)
	if err := q.EnsureSyncState(context.Background(), dbgen.EnsureSyncStateParams{
		TargetKey: DefaultSyncTargetKey, SyncTypeID: DefaultSyncTypeID, Lifecycle: SyncLifecycleIdle,
	}); err != nil {
		return err
	}
	seq, err := q.InsertSyncMutation(context.Background(), dbgen.InsertSyncMutationParams{
		TargetKey: DefaultSyncTargetKey, Entity: entity, EntityKey: entityKey, Op: op,
		Payload: string(encoded), Source: SyncSourceLocal,
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
	if err := s.ensureProjectTx(tx, payload.Project); err != nil {
		return err
	}
	if err := q.ApplySessionPayload(context.Background(), dbgen.ApplySessionPayloadParams{
		ID: payload.ID, Project: payload.Project, Directory: payload.Directory,
		EndedAt: sqlNullStringPtr(payload.EndedAt), Summary: sqlNullStringPtr(payload.Summary),
		IsDeleted: boolToInt64(payload.IsDeleted), ProvenanceID: provenanceID,
	}); err != nil {
		return err
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
		if !hasProvenanceInput(provenanceInput) {
			provenanceInput = SyncPullProvenance()
		}
		provenanceID, err := s.optionalProvenanceTx(tx, provenanceInput)
		if err != nil {
			return err
		}
		_, err = q.InsertPulledObservation(context.Background(), dbgen.InsertPulledObservationParams{
			SyncID: sqlNullString(payload.SyncID), SessionID: payload.SessionID, Type: payload.Type,
			Title: payload.Title, Content: payload.Content, ToolName: sqlNullStringPtr(payload.ToolName),
			Scope:    normalizeScope(payload.Scope),
			TopicKey: sqlNullStringPtr(payload.TopicKey), NormalizedHash: sqlNullString(hashNormalized(payload.Content)),
			IsDeleted: boolToInt64(payload.IsDeleted), ProvenanceID: provenanceID,
		})
		if err != nil {
			return err
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
		ToolName: sqlNullStringPtr(payload.ToolName),
		Scope:    normalizeScope(payload.Scope), TopicKey: sqlNullStringPtr(payload.TopicKey),
		NormalizedHash: sqlNullString(hashNormalized(payload.Content)), IsDeleted: boolToInt64(payload.IsDeleted),
		ProvenanceID: provenanceID, ID: existing.ID,
	})
	if err != nil {
		return err
	}
	return nil
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
			Content: payload.Content, IsDeleted: boolToInt64(payload.IsDeleted), ProvenanceID: provenanceID,
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
		IsDeleted: boolToInt64(payload.IsDeleted), ProvenanceID: provenanceID, ID: existingID,
	})
}

// BackfillAllSyncMutations reconstructs missing queue entries from every canonical
// synchronizable table, including soft-deleted rows.
func (s *Store) BackfillAllSyncMutations() error {
	return s.withTx(func(tx *sql.Tx) error {
		if err := s.ensureSyncStateTx(tx, DefaultSyncTargetKey); err != nil {
			return err
		}
		// The canonical tables are the source of truth. The queue is deliberately
		// reconstructed from every row, including soft-deleted rows.
		for _, table := range []string{"projects", "sessions", "observations", "user_prompts", "observation_tags", "session_tags", "observation_reviews", "provenance_contexts", "agents", "tools", "models", "source_kinds", "mcp_clients"} {
			if err := s.backfillCanonicalTableTx(tx, table); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) backfillCanonicalTableTx(tx *sql.Tx, table string) error {
	queries := map[string]string{
		"projects":            `SELECT id, name, is_deleted FROM projects ORDER BY id`,
		"sessions":            `SELECT id, project, directory, ended_at, summary, is_deleted FROM sessions ORDER BY id`,
		"observations":        `SELECT ifnull(sync_id,''), session_id, type, title, content, tool_name, scope, topic_key, is_deleted FROM observations ORDER BY id`,
		"user_prompts":        `SELECT ifnull(sync_id,''), session_id, content, is_deleted FROM user_prompts ORDER BY id`,
		"observation_tags":    `SELECT ifnull(o.sync_id,''), t.tag, t.is_deleted FROM observation_tags t JOIN observations o ON o.id=t.observation_id ORDER BY o.sync_id, t.tag`,
		"session_tags":        `SELECT session_id, tag, is_deleted FROM session_tags ORDER BY session_id, tag`,
		"observation_reviews": `SELECT ifnull(o.sync_id,''), r.state, r.reason, r.superseded_by, r.reviewed_at, r.is_deleted FROM observation_reviews r JOIN observations o ON o.id=r.observation_id ORDER BY o.sync_id`,
		"provenance_contexts": `SELECT id, agent_id, source_kind_id, tool_id, model_id, mcp_client_id, is_deleted FROM provenance_contexts ORDER BY id`,
		"agents":              `SELECT id, display_name, kind, is_deleted FROM agents WHERE EXISTS (SELECT 1 FROM provenance_contexts p WHERE p.agent_id=agents.id) ORDER BY id`,
		"tools":               `SELECT id, display_name, is_deleted FROM tools WHERE EXISTS (SELECT 1 FROM provenance_contexts p WHERE p.tool_id=tools.id) ORDER BY id`,
		"models":              `SELECT id, provider, display_name, is_deleted FROM models WHERE EXISTS (SELECT 1 FROM provenance_contexts p WHERE p.model_id=models.id) ORDER BY id`,
		"source_kinds":        `SELECT id, display_name, is_deleted FROM source_kinds WHERE EXISTS (SELECT 1 FROM provenance_contexts p WHERE p.source_kind_id=source_kinds.id) ORDER BY id`,
		"mcp_clients":         `SELECT id, name, version, transport, is_deleted FROM mcp_clients WHERE EXISTS (SELECT 1 FROM provenance_contexts p WHERE p.mcp_client_id=mcp_clients.id) ORDER BY id`,
	}
	query, ok := queries[table]
	if !ok {
		return fmt.Errorf("unknown canonical sync table %q", table)
	}
	rows, err := s.queryItHook(tx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var payload any
		switch table {
		case "projects":
			var name string
			var deleted int64
			if err := rows.Scan(&key, &name, &deleted); err != nil {
				return err
			}
			payload = map[string]any{"id": key, "name": name, "is_deleted": int64ToBool(deleted)}
		case "sessions":
			var project, directory string
			var ended, summary sql.NullString
			var deleted int64
			if err := rows.Scan(&key, &project, &directory, &ended, &summary, &deleted); err != nil {
				return err
			}
			payload = map[string]any{"id": key, "project": project, "directory": directory, "ended_at": nullablePtr(ended), "summary": nullablePtr(summary), "is_deleted": int64ToBool(deleted)}
		case "observations":
			var session, typ, title, content string
			var tool, scope, topic sql.NullString
			var deleted int64
			if err := rows.Scan(&key, &session, &typ, &title, &content, &tool, &scope, &topic, &deleted); err != nil {
				return err
			}
			payload = map[string]any{"sync_id": key, "session_id": session, "type": typ, "title": title, "content": content, "tool_name": nullablePtr(tool), "scope": scope.String, "topic_key": nullablePtr(topic), "is_deleted": int64ToBool(deleted)}
		case "user_prompts":
			var session, content string
			var deleted int64
			if err := rows.Scan(&key, &session, &content, &deleted); err != nil {
				return err
			}
			payload = map[string]any{"sync_id": key, "session_id": session, "content": content, "is_deleted": int64ToBool(deleted)}
		case "observation_tags":
			var obs string
			var tag string
			var deleted int64
			if err := rows.Scan(&obs, &tag, &deleted); err != nil {
				return err
			}
			key = obs + ":" + tag
			payload = map[string]any{"observation_sync_id": obs, "tag": tag, "is_deleted": int64ToBool(deleted)}
		case "session_tags":
			var session, tag string
			var deleted int64
			if err := rows.Scan(&session, &tag, &deleted); err != nil {
				return err
			}
			key = session + ":" + tag
			payload = map[string]any{"session_id": session, "tag": tag, "is_deleted": int64ToBool(deleted)}
		case "observation_reviews":
			var obs string
			var state, reason string
			var superseded sql.NullInt64
			var reviewed sql.NullString
			var deleted int64
			if err := rows.Scan(&obs, &state, &reason, &superseded, &reviewed, &deleted); err != nil {
				return err
			}
			key = obs
			payload = map[string]any{"observation_sync_id": obs, "state": state, "reason": reason, "superseded_by": nullableInt64(superseded), "reviewed_at": nullablePtr(reviewed), "is_deleted": int64ToBool(deleted)}
		case "provenance_contexts":
			var id int64
			var agent, source, tool, model, mcp string
			var deleted int64
			if err := rows.Scan(&id, &agent, &source, &tool, &model, &mcp, &deleted); err != nil {
				return err
			}
			key = fmt.Sprintf("%d", id)
			payload = map[string]any{"id": id, "agent_id": agent, "source_kind_id": source, "tool_id": tool, "model_id": model, "mcp_client_id": mcp, "is_deleted": int64ToBool(deleted)}
		case "agents", "source_kinds", "tools":
			var display string
			var deleted int64
			if table == "tools" {
				if err := rows.Scan(&key, &display, &deleted); err != nil {
					return err
				}
				payload = map[string]any{"id": key, "display_name": display, "is_deleted": int64ToBool(deleted)}
			} else {
				var kind string
				if table == "agents" {
					if err := rows.Scan(&key, &display, &kind, &deleted); err != nil {
						return err
					}
					payload = map[string]any{"id": key, "display_name": display, "kind": kind, "is_deleted": int64ToBool(deleted)}
				} else {
					if err := rows.Scan(&key, &display, &deleted); err != nil {
						return err
					}
					payload = map[string]any{"id": key, "display_name": display, "is_deleted": int64ToBool(deleted)}
				}
			}
		case "models":
			var provider, display string
			var deleted int64
			if err := rows.Scan(&key, &provider, &display, &deleted); err != nil {
				return err
			}
			payload = map[string]any{"id": key, "provider": provider, "display_name": display, "is_deleted": int64ToBool(deleted)}
		case "mcp_clients":
			var name, version, transport string
			var deleted int64
			if err := rows.Scan(&key, &name, &version, &transport, &deleted); err != nil {
				return err
			}
			payload = map[string]any{"id": key, "name": name, "version": version, "transport": transport, "is_deleted": int64ToBool(deleted)}
		}
		missing, err := s.queryItHook(tx, `SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM sync_mutations WHERE target_key = ? AND entity = ? AND entity_key = ? AND source = ?)`, DefaultSyncTargetKey, syncEntityForTable(table), key, SyncSourceLocal)
		if err != nil {
			return err
		}
		if missing.Next() {
			_ = missing.Close()
			if err := s.enqueueSyncMutationTx(tx, syncEntityForTable(table), key, SyncOpUpsert, payload); err != nil {
				return err
			}
		} else {
			_ = missing.Close()
		}
	}
	return rows.Err()
}

func syncEntityForTable(table string) string {
	if table == "user_prompts" {
		return SyncEntityUserPrompt
	}
	return map[string]string{"projects": SyncEntityProject, "sessions": SyncEntitySession, "observations": SyncEntityObservation, "observation_tags": SyncEntityObservationTag, "session_tags": SyncEntitySessionTag, "observation_reviews": SyncEntityObservationReview, "provenance_contexts": SyncEntityProvenanceContext, "agents": SyncEntityAgent, "tools": SyncEntityTool, "models": SyncEntityModel, "source_kinds": SyncEntitySourceKind, "mcp_clients": SyncEntityMCPClient}[table]
}

// ListAllPendingSyncMutations returns pending local mutations for a sync target.
func (s *Store) ListAllPendingSyncMutations(targetKey string, limit int) ([]SyncMutation, error) {
	targetKey = normalizeSyncTargetKey(targetKey)
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.queryItHook(s.db, `
		SELECT seq, target_key, entity, entity_key, op, payload, source, occurred_at, acked_at
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
		if err := rows.Scan(&mutation.Seq, &mutation.TargetKey, &mutation.Entity, &mutation.EntityKey, &mutation.Op, &mutation.Payload, &mutation.Source, &mutation.OccurredAt, &mutation.AckedAt); err != nil {
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
