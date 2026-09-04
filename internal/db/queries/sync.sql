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
