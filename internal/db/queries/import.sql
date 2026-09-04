-- name: ImportSession :execrows
INSERT OR IGNORE INTO sessions (id, project, directory, started_at, ended_at, summary, provenance_id)
VALUES (
  sqlc.arg('id'), sqlc.arg('project'), sqlc.arg('directory'), sqlc.arg('started_at'),
  sqlc.narg('ended_at'), sqlc.narg('summary'), sqlc.narg('provenance_id')
);

-- name: ImportObservation :one
INSERT INTO observations (
  sync_id, session_id, type, title, content, tool_name, scope, topic_key,
  normalized_hash, revision_count, duplicate_count, last_seen_at, created_at, updated_at, is_deleted,
  provenance_id
) VALUES (
  sqlc.narg('sync_id'), sqlc.arg('session_id'), sqlc.arg('type'), sqlc.arg('title'),
  sqlc.arg('content'), sqlc.narg('tool_name'), sqlc.arg('scope'),
  sqlc.narg('topic_key'), sqlc.narg('normalized_hash'), sqlc.arg('revision_count'),
  sqlc.arg('duplicate_count'), sqlc.narg('last_seen_at'), sqlc.arg('created_at'),
  sqlc.arg('updated_at'), sqlc.arg('is_deleted'), sqlc.narg('provenance_id')
)
RETURNING id;

-- name: ImportPrompt :exec
INSERT INTO user_prompts (sync_id, session_id, content, created_at, is_deleted, provenance_id)
VALUES (
  sqlc.narg('sync_id'), sqlc.arg('session_id'), sqlc.arg('content'),
  sqlc.arg('created_at'), sqlc.arg('is_deleted'), sqlc.narg('provenance_id')
);
