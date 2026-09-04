-- name: ApplySessionPayload :exec
INSERT INTO sessions (id, project, directory, ended_at, summary, is_deleted, provenance_id)
VALUES (
  sqlc.arg('id'), sqlc.arg('project'), sqlc.arg('directory'),
  sqlc.narg('ended_at'), sqlc.narg('summary'), sqlc.arg('is_deleted'), sqlc.narg('provenance_id')
)
ON CONFLICT(id) DO UPDATE SET
  project = excluded.project,
  directory = excluded.directory,
  ended_at = COALESCE(excluded.ended_at, sessions.ended_at),
  summary = COALESCE(excluded.summary, sessions.summary),
  is_deleted = excluded.is_deleted,
  provenance_id = COALESCE(excluded.provenance_id, sessions.provenance_id);

-- name: InsertPulledObservation :one
INSERT INTO observations (
  sync_id, session_id, type, title, content, tool_name, scope, topic_key,
  normalized_hash, revision_count, duplicate_count, updated_at, is_deleted, provenance_id
) VALUES (
  sqlc.narg('sync_id'), sqlc.arg('session_id'), sqlc.arg('type'), sqlc.arg('title'),
  sqlc.arg('content'), sqlc.narg('tool_name'), sqlc.arg('scope'),
  sqlc.narg('topic_key'), sqlc.narg('normalized_hash'), 1, 1, datetime('now'), sqlc.arg('is_deleted'),
  sqlc.narg('provenance_id')
)
RETURNING id;

-- name: UpdatePulledObservation :exec
UPDATE observations SET
  session_id = sqlc.arg('session_id'),
  type = sqlc.arg('type'),
  title = sqlc.arg('title'),
  content = sqlc.arg('content'),
  tool_name = sqlc.narg('tool_name'),
  scope = sqlc.arg('scope'),
  topic_key = sqlc.narg('topic_key'),
  normalized_hash = sqlc.narg('normalized_hash'),
  provenance_id = COALESCE(sqlc.narg('provenance_id'), provenance_id),
  revision_count = revision_count + 1,
  updated_at = datetime('now'),
  is_deleted = sqlc.arg('is_deleted')
WHERE id = sqlc.arg('id');
