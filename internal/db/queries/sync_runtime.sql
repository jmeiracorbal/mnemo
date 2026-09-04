-- name: ListPendingSyncMutations :many
SELECT sm.seq, sm.target_key, sm.entity, sm.entity_key, sm.op, sm.payload, sm.source,
       sm.occurred_at, sm.acked_at
FROM sync_mutations sm
WHERE sm.target_key = sqlc.arg('target_key') AND sm.acked_at IS NULL
ORDER BY sm.seq ASC
LIMIT sqlc.arg('result_limit');

-- name: AckMutationsThrough :exec
UPDATE sync_mutations
SET acked_at = datetime('now')
WHERE target_key = sqlc.arg('target_key')
  AND seq <= sqlc.arg('last_acked_seq')
  AND acked_at IS NULL;

-- name: AckMutationSeq :exec
UPDATE sync_mutations
SET acked_at = datetime('now')
WHERE target_key = sqlc.arg('target_key')
  AND seq = sqlc.arg('seq')
  AND acked_at IS NULL;

-- name: CountPendingMutations :one
SELECT COUNT(*) FROM sync_mutations
WHERE target_key = ? AND acked_at IS NULL;

-- name: UpdateSyncAckState :exec
UPDATE sync_state
SET last_acked_seq = sqlc.arg('last_acked_seq'),
    lifecycle = sqlc.arg('lifecycle'),
    updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key');

-- name: AcquireSyncLease :execrows
UPDATE sync_state
SET lease_owner = sqlc.arg('owner'),
    lease_until = sqlc.arg('lease_until'),
    lifecycle = sqlc.arg('lifecycle'),
    updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key')
  AND (
    lease_owner IS NULL OR lease_until IS NULL OR
    datetime(lease_until) <= datetime(sqlc.arg('now')) OR
    lease_owner = sqlc.arg('owner')
  );

-- name: ReleaseSyncLease :exec
UPDATE sync_state
SET lease_owner = NULL, lease_until = NULL, updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key')
  AND (lease_owner = sqlc.arg('owner') OR lease_owner IS NULL OR lease_owner = '');

-- name: MarkSyncFailure :exec
UPDATE sync_state
SET lifecycle = sqlc.arg('lifecycle'),
    consecutive_failures = consecutive_failures + 1,
    backoff_until = sqlc.arg('backoff_until'),
    last_error = sqlc.arg('last_error'),
    updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key');

-- name: MarkSyncHealthy :exec
UPDATE sync_state
SET lifecycle = sqlc.arg('lifecycle'),
    consecutive_failures = 0,
    backoff_until = NULL,
    last_error = NULL,
    updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key');

-- name: UpdateLastPulledSeq :exec
UPDATE sync_state
SET last_pulled_seq = sqlc.arg('last_pulled_seq'),
    lifecycle = sqlc.arg('lifecycle'),
    consecutive_failures = 0,
    backoff_until = NULL,
    last_error = NULL,
    updated_at = datetime('now')
WHERE target_key = sqlc.arg('target_key');

-- name: ProjectExists :one
SELECT EXISTS(
  SELECT 1 FROM projects pr WHERE pr.id = sqlc.arg('project_name') AND pr.is_deleted = 0
  UNION SELECT 1 FROM sessions s WHERE s.project = sqlc.arg('project_name') AND s.is_deleted = 0
);

-- name: CountObservationProjectRows :one
SELECT COUNT(*)
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE s.project = sqlc.arg('project_name');

-- name: CountSessionProjectRows :one
SELECT COUNT(*) FROM sessions WHERE project = sqlc.arg('project_name');

-- name: CountPromptProjectRows :one
SELECT COUNT(*)
FROM user_prompts p
JOIN sessions s ON s.id = p.session_id
WHERE s.project = sqlc.arg('project_name');

-- name: RenameSessionProject :execrows
UPDATE sessions SET project = sqlc.arg('new_name') WHERE project = sqlc.arg('old_name');
