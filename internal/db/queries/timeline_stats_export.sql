-- name: ListTimelineBefore :many
SELECT id, session_id, type, title, content, tool_name, project,
       scope, topic_key, revision_count, duplicate_count, last_seen_at,
       created_at, updated_at, deleted_at
FROM observations
WHERE session_id = sqlc.arg('session_id')
  AND id < sqlc.arg('observation_id')
  AND deleted_at IS NULL
ORDER BY id DESC
LIMIT sqlc.arg('result_limit');

-- name: ListTimelineAfter :many
SELECT id, session_id, type, title, content, tool_name, project,
       scope, topic_key, revision_count, duplicate_count, last_seen_at,
       created_at, updated_at, deleted_at
FROM observations
WHERE session_id = sqlc.arg('session_id')
  AND id > sqlc.arg('observation_id')
  AND deleted_at IS NULL
ORDER BY id ASC
LIMIT sqlc.arg('result_limit');

-- name: CountSessions :one
SELECT COUNT(*) FROM sessions;

-- name: CountLiveObservations :one
SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL;

-- name: CountPrompts :one
SELECT COUNT(*) FROM user_prompts;

-- name: ListObservationProjects :many
SELECT project
FROM observations
WHERE project IS NOT NULL AND deleted_at IS NULL
GROUP BY project
ORDER BY MAX(created_at) DESC;

-- name: ExportSessions :many
SELECT id, project, directory, started_at, ended_at, summary
FROM sessions ORDER BY started_at;

-- name: ExportObservations :many
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, type, title, content,
       tool_name, project, scope, topic_key, revision_count, duplicate_count,
       last_seen_at, created_at, updated_at, deleted_at
FROM observations ORDER BY id;

-- name: ExportPrompts :many
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, content,
       ifnull(project, '') AS project, created_at
FROM user_prompts ORDER BY id;

