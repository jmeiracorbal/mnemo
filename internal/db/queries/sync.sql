-- name: EnsureSyncType :exec
INSERT OR IGNORE INTO sync_types (id, display_name) VALUES (?, ?);

-- name: EnsureSyncState :exec
INSERT OR IGNORE INTO sync_state (target_key, sync_type_id, lifecycle, updated_at)
VALUES (?, ?, ?, datetime('now'));

-- name: GetSyncState :one
SELECT target_key, sync_type_id, lifecycle, last_enqueued_seq, last_acked_seq, last_pulled_seq,
       consecutive_failures, backoff_until, lease_owner, lease_until, last_error, updated_at
FROM sync_state WHERE target_key = ?;

-- name: InsertSyncMutation :one
INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING seq;

-- name: UpdateLastEnqueuedSeq :exec
UPDATE sync_state
SET last_enqueued_seq = ?, lifecycle = ?, updated_at = datetime('now')
WHERE target_key = ?;

-- name: ListPendingSyncMutations :many
SELECT seq, target_key, entity, entity_key, op, payload, source, occurred_at, acked_at
FROM sync_mutations
WHERE target_key = ? AND acked_at IS NULL
ORDER BY CASE entity
    WHEN 'project' THEN 10
    WHEN 'agent' THEN 20
    WHEN 'tool' THEN 20
    WHEN 'model' THEN 20
    WHEN 'source_kind' THEN 20
    WHEN 'mcp_client' THEN 20
    WHEN 'provenance_context' THEN 30
    WHEN 'session' THEN 40
    WHEN 'observation' THEN 50
    WHEN 'user_prompt' THEN 50
    WHEN 'session_tag' THEN 60
    WHEN 'observation_tag' THEN 60
    WHEN 'observation_review' THEN 60
    ELSE 100
END ASC, seq ASC
LIMIT ?;

-- name: ListSyncMutationPayloads :many
SELECT seq, source, payload, acked_at
FROM sync_mutations
WHERE target_key = ? AND entity = ? AND entity_key = ?;

-- name: AdvanceSyncAckedSeq :exec
UPDATE sync_state
SET last_acked_seq = CASE WHEN last_acked_seq < sqlc.arg('last_acked_seq')
    THEN sqlc.arg('last_acked_seq') ELSE last_acked_seq END,
    updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key');
