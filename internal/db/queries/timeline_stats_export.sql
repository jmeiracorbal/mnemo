-- name: ListTimelineBefore :many
SELECT o.id, o.session_id, o.type, o.title, o.content, o.tool_name, s.project AS project,
       o.scope, o.topic_key, o.revision_count, o.duplicate_count, o.last_seen_at,
       o.created_at, o.updated_at, o.is_deleted, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.session_id = sqlc.arg('session_id')
  AND o.id < sqlc.arg('observation_id')
  AND o.is_deleted = 0
ORDER BY o.id DESC
LIMIT sqlc.arg('result_limit');

-- name: ListTimelineAfter :many
SELECT o.id, o.session_id, o.type, o.title, o.content, o.tool_name, s.project AS project,
       o.scope, o.topic_key, o.revision_count, o.duplicate_count, o.last_seen_at,
       o.created_at, o.updated_at, o.is_deleted, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.session_id = sqlc.arg('session_id')
  AND o.id > sqlc.arg('observation_id')
  AND o.is_deleted = 0
ORDER BY o.id ASC
LIMIT sqlc.arg('result_limit');

-- name: CountSessions :one
SELECT COUNT(*) FROM sessions;

-- name: CountLiveObservations :one
SELECT COUNT(*) FROM observations WHERE is_deleted = 0;

-- name: CountPrompts :one
SELECT COUNT(*) FROM user_prompts;

-- name: ListObservationProjects :many
SELECT s.project
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.is_deleted = 0
GROUP BY s.project
ORDER BY MAX(o.created_at) DESC;

-- name: ExportSessions :many
SELECT id, project, directory, started_at, ended_at, summary, provenance_id
FROM sessions ORDER BY started_at;

-- name: ExportObservations :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.is_deleted, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
ORDER BY o.id;

-- name: ExportPrompts :many
SELECT p.id, ifnull(p.sync_id, '') AS sync_id, p.session_id, p.content,
       s.project AS project, p.created_at, p.is_deleted, p.provenance_id
FROM user_prompts p
JOIN sessions s ON s.id = p.session_id
ORDER BY p.id;
