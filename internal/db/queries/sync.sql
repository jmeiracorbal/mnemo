-- name: EnsureSyncState :exec
INSERT OR IGNORE INTO sync_state (target_key, lifecycle, updated_at)
VALUES (?, ?, datetime('now'));

-- name: GetSyncState :one
SELECT target_key, lifecycle, last_enqueued_seq, last_acked_seq, last_pulled_seq,
       consecutive_failures, backoff_until, lease_owner, lease_until, last_error, updated_at
FROM sync_state WHERE target_key = ?;

-- name: InsertSyncMutation :one
INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source, project)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING seq;

-- name: UpdateLastEnqueuedSeq :exec
UPDATE sync_state
SET last_enqueued_seq = ?, lifecycle = ?, updated_at = datetime('now')
WHERE target_key = ?;

-- name: InsertSyncedChunk :exec
INSERT OR IGNORE INTO sync_chunks (chunk_id) VALUES (?);

-- name: ListSyncedChunks :many
SELECT chunk_id FROM sync_chunks;

-- name: EnrollProject :execrows
INSERT OR IGNORE INTO sync_enrolled_projects (project) VALUES (?);

-- name: UnenrollProject :exec
DELETE FROM sync_enrolled_projects WHERE project = ?;

-- name: ListEnrolledProjects :many
SELECT project, enrolled_at FROM sync_enrolled_projects ORDER BY project ASC;

-- name: IsProjectEnrolled :one
SELECT EXISTS(SELECT 1 FROM sync_enrolled_projects WHERE project = ?);
