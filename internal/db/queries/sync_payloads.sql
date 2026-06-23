-- name: ListSessionsMissingSyncMutation :many
SELECT id, project, directory, ended_at, summary
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
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, type, title, content,
       tool_name, project, scope, topic_key
FROM observations
WHERE ifnull(observations.project, '') = sqlc.arg('project_name')
  AND deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sync_mutations sm
    WHERE sm.target_key = sqlc.arg('target_key')
      AND sm.entity = sqlc.arg('entity')
      AND sm.entity_key = ifnull(observations.sync_id, '')
      AND sm.source = sqlc.arg('source')
  )
ORDER BY id ASC;

-- name: ListPromptsMissingSyncMutation :many
SELECT ifnull(sync_id, '') AS sync_id, session_id, content, project
FROM user_prompts
WHERE ifnull(user_prompts.project, '') = sqlc.arg('project_name')
  AND NOT EXISTS (
    SELECT 1 FROM sync_mutations sm
    WHERE sm.target_key = sqlc.arg('target_key')
      AND sm.entity = sqlc.arg('entity')
      AND sm.entity_key = ifnull(user_prompts.sync_id, '')
      AND sm.source = sqlc.arg('source')
  )
ORDER BY id ASC;

-- name: ApplySessionPayload :exec
INSERT INTO sessions (id, project, directory, ended_at, summary)
VALUES (
  sqlc.arg('id'), sqlc.arg('project'), sqlc.arg('directory'),
  sqlc.narg('ended_at'), sqlc.narg('summary')
)
ON CONFLICT(id) DO UPDATE SET
  project = excluded.project,
  directory = excluded.directory,
  ended_at = COALESCE(excluded.ended_at, sessions.ended_at),
  summary = COALESCE(excluded.summary, sessions.summary);

-- name: InsertPulledObservation :one
INSERT INTO observations (
  sync_id, session_id, type, title, content, tool_name, project, scope, topic_key,
  normalized_hash, revision_count, duplicate_count, updated_at, deleted_at
) VALUES (
  sqlc.narg('sync_id'), sqlc.arg('session_id'), sqlc.arg('type'), sqlc.arg('title'),
  sqlc.arg('content'), sqlc.narg('tool_name'), sqlc.narg('project'), sqlc.arg('scope'),
  sqlc.narg('topic_key'), sqlc.narg('normalized_hash'), 1, 1, datetime('now'), NULL
)
RETURNING id;

-- name: UpdatePulledObservation :exec
UPDATE observations SET
  session_id = sqlc.arg('session_id'),
  type = sqlc.arg('type'),
  title = sqlc.arg('title'),
  content = sqlc.arg('content'),
  tool_name = sqlc.narg('tool_name'),
  project = sqlc.narg('project'),
  scope = sqlc.arg('scope'),
  topic_key = sqlc.narg('topic_key'),
  normalized_hash = sqlc.narg('normalized_hash'),
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
