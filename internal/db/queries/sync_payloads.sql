-- name: ListSessionsMissingSyncMutation :many
SELECT id, project, directory, ended_at, summary, provenance_id
FROM sessions
WHERE sessions.project = sqlc.arg('project_name')
  AND NOT EXISTS (
    SELECT 1 FROM sync_mutations sm
    WHERE sm.target_key = sqlc.arg('target_key')
      AND sm.entity = sqlc.arg('entity')
      AND sm.entity_key = sessions.id
      AND sm.source = sqlc.arg('source')
  )
ORDER BY started_at ASC, id ASC;

-- name: ListObservationsMissingSyncMutation :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE s.project = sqlc.arg('project_name')
  AND o.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sync_mutations sm
    WHERE sm.target_key = sqlc.arg('target_key')
      AND sm.entity = sqlc.arg('entity')
      AND sm.entity_key = ifnull(o.sync_id, '')
      AND sm.source = sqlc.arg('source')
  )
ORDER BY o.id ASC;

-- name: ListPromptsMissingSyncMutation :many
SELECT ifnull(p.sync_id, '') AS sync_id, p.session_id, p.content, s.project AS project, p.provenance_id
FROM user_prompts p
JOIN sessions s ON s.id = p.session_id
WHERE s.project = sqlc.arg('project_name')
  AND NOT EXISTS (
    SELECT 1 FROM sync_mutations sm
    WHERE sm.target_key = sqlc.arg('target_key')
      AND sm.entity = sqlc.arg('entity')
      AND sm.entity_key = ifnull(p.sync_id, '')
      AND sm.source = sqlc.arg('source')
  )
ORDER BY p.id ASC;

-- name: ApplySessionPayload :exec
INSERT INTO sessions (id, project, directory, ended_at, summary, provenance_id)
VALUES (
  sqlc.arg('id'), sqlc.arg('project'), sqlc.arg('directory'),
  sqlc.narg('ended_at'), sqlc.narg('summary'), sqlc.narg('provenance_id')
)
ON CONFLICT(id) DO UPDATE SET
  project = excluded.project,
  directory = excluded.directory,
  ended_at = COALESCE(excluded.ended_at, sessions.ended_at),
  summary = COALESCE(excluded.summary, sessions.summary),
  provenance_id = COALESCE(excluded.provenance_id, sessions.provenance_id);

-- name: InsertPulledObservation :one
INSERT INTO observations (
  sync_id, session_id, type, title, content, tool_name, scope, topic_key,
  normalized_hash, revision_count, duplicate_count, updated_at, deleted_at, provenance_id
) VALUES (
  sqlc.narg('sync_id'), sqlc.arg('session_id'), sqlc.arg('type'), sqlc.arg('title'),
  sqlc.arg('content'), sqlc.narg('tool_name'), sqlc.arg('scope'),
  sqlc.narg('topic_key'), sqlc.narg('normalized_hash'), 1, 1, datetime('now'), NULL,
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
  deleted_at = NULL
WHERE id = sqlc.arg('id');

-- name: DeleteObservationByID :exec
DELETE FROM observations WHERE id = ?;

-- name: SetObservationDeletedAt :exec
UPDATE observations
SET deleted_at = sqlc.narg('deleted_at'), updated_at = datetime('now')
WHERE id = sqlc.arg('id');
