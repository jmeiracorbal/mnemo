-- name: ListPendingSyncMutations :many
SELECT sm.seq, sm.target_key, sm.entity, sm.entity_key, sm.op, sm.payload, sm.source, sm.project,
       sm.occurred_at, sm.acked_at
FROM sync_mutations sm
LEFT JOIN sync_enrolled_projects sep ON sm.project = sep.project
WHERE sm.target_key = sqlc.arg('target_key') AND sm.acked_at IS NULL
  AND (sm.project = '' OR sep.project IS NOT NULL)
ORDER BY sm.seq ASC
LIMIT sqlc.arg('result_limit');

-- name: SkipNonEnrolledMutations :execrows
UPDATE sync_mutations
SET acked_at = datetime('now')
WHERE target_key = sqlc.arg('target_key')
  AND acked_at IS NULL
  AND project != ''
  AND project NOT IN (SELECT project FROM sync_enrolled_projects);

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
  SELECT 1 FROM observations o WHERE o.project = sqlc.arg('project_name')
  UNION SELECT 1 FROM sessions s WHERE s.project = sqlc.arg('project_name')
  UNION SELECT 1 FROM user_prompts p WHERE p.project = sqlc.arg('project_name')
  UNION SELECT 1 FROM sync_mutations m WHERE m.project = sqlc.arg('project_name')
  UNION SELECT 1 FROM sync_enrolled_projects e WHERE e.project = sqlc.arg('project_name')
);

-- name: RenameObservationProject :execrows
UPDATE observations SET project = sqlc.arg('new_name') WHERE project = sqlc.arg('old_name');

-- name: RenameSessionProject :execrows
UPDATE sessions SET project = sqlc.arg('new_name') WHERE project = sqlc.arg('old_name');

-- name: RenamePromptProject :execrows
UPDATE user_prompts SET project = sqlc.arg('new_name') WHERE project = sqlc.arg('old_name');

-- name: RenameMutationProject :execrows
UPDATE sync_mutations
SET project = sqlc.arg('new_name'),
    payload = json_set(payload, '$.project', sqlc.arg('new_name'))
WHERE project = sqlc.arg('old_name');

-- name: CopyProjectEnrollment :exec
INSERT OR IGNORE INTO sync_enrolled_projects (project, enrolled_at)
SELECT sqlc.arg('new_name'), enrolled_at
FROM sync_enrolled_projects
WHERE sync_enrolled_projects.project = sqlc.arg('old_name');

-- name: DeleteProjectEnrollment :exec
DELETE FROM sync_enrolled_projects WHERE project = ?;
