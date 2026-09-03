-- name: GetObservation :one
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.deleted_at, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.id = ?;

-- name: GetLiveObservation :one
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.deleted_at, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.id = ? AND o.deleted_at IS NULL;

-- name: GetLiveObservationBySyncID :one
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.deleted_at, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.sync_id = ? AND o.deleted_at IS NULL
ORDER BY o.id DESC LIMIT 1;

-- name: GetObservationBySyncIDIncludingDeleted :one
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.deleted_at, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.sync_id = ?
ORDER BY o.id DESC LIMIT 1;

-- name: FindObservationByTopic :one
SELECT o.id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.topic_key = sqlc.arg('topic_key')
  AND s.project = sqlc.arg('project')
  AND o.scope = sqlc.arg('scope')
  AND o.deleted_at IS NULL
ORDER BY datetime(o.updated_at) DESC, datetime(o.created_at) DESC
LIMIT 1;

-- name: FindDuplicateObservation :one
SELECT o.id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.normalized_hash = sqlc.arg('normalized_hash')
  AND s.project = sqlc.arg('project')
  AND o.scope = sqlc.arg('scope')
  AND o.type = sqlc.arg('type')
  AND o.title = sqlc.arg('title')
  AND o.deleted_at IS NULL
  AND datetime(o.created_at) >= datetime('now', sqlc.arg('window'))
ORDER BY o.id DESC LIMIT 1;

-- name: InsertObservation :one
INSERT INTO observations (
  sync_id, session_id, type, title, content, tool_name, scope,
  topic_key, normalized_hash, revision_count, duplicate_count, last_seen_at, updated_at, provenance_id
) VALUES (
  sqlc.arg('sync_id'), sqlc.arg('session_id'), sqlc.arg('type'), sqlc.arg('title'),
  sqlc.arg('content'), sqlc.narg('tool_name'), sqlc.arg('scope'),
  sqlc.narg('topic_key'), sqlc.arg('normalized_hash'), 1, 1, datetime('now'), datetime('now'),
  sqlc.narg('provenance_id')
)
RETURNING id;

-- name: UpdateObservationByTopic :exec
UPDATE observations SET
  type = sqlc.arg('type'),
  title = sqlc.arg('title'),
  content = sqlc.arg('content'),
  tool_name = sqlc.narg('tool_name'),
  topic_key = sqlc.narg('topic_key'),
  normalized_hash = sqlc.arg('normalized_hash'),
  provenance_id = COALESCE(sqlc.narg('provenance_id'), provenance_id),
  revision_count = revision_count + 1,
  last_seen_at = datetime('now'),
  updated_at = datetime('now')
WHERE id = sqlc.arg('id');

-- name: UpdateObservationFields :exec
UPDATE observations SET
  type = sqlc.arg('type'),
  title = sqlc.arg('title'),
  content = sqlc.arg('content'),
  scope = sqlc.arg('scope'),
  topic_key = sqlc.narg('topic_key'),
  normalized_hash = sqlc.arg('normalized_hash'),
  revision_count = revision_count + 1,
  updated_at = datetime('now')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: TouchDuplicateObservation :exec
UPDATE observations SET
  duplicate_count = duplicate_count + 1,
  last_seen_at = datetime('now'),
  updated_at = datetime('now')
WHERE id = ?;

-- name: SoftDeleteObservation :exec
UPDATE observations
SET deleted_at = datetime('now'), updated_at = datetime('now')
WHERE id = ?;

-- name: GetObservationDeletedAt :one
SELECT deleted_at FROM observations WHERE id = ?;

-- name: HardDeleteObservation :exec
DELETE FROM observations WHERE id = ?;

-- name: DeleteObservationTags :exec
DELETE FROM observation_tags WHERE observation_id = ?;

-- name: InsertObservationTag :exec
INSERT OR IGNORE INTO observation_tags (observation_id, tag) VALUES (?, ?);

-- name: ListObservationTags :many
SELECT tag FROM observation_tags WHERE observation_id = ? ORDER BY tag;

-- name: ListTagsForObservationIDs :many
SELECT observation_id, tag
FROM observation_tags
WHERE observation_id IN (sqlc.slice('observation_ids'))
ORDER BY observation_id, tag;

-- name: CountObservationsByHash :one
SELECT COUNT(*) FROM observations
WHERE normalized_hash = ? AND deleted_at IS NULL;

-- name: FindObservationByHashAndProject :one
SELECT o.id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.normalized_hash = sqlc.narg('normalized_hash')
  AND s.project = sqlc.arg('project')
  AND o.deleted_at IS NULL
LIMIT 1;
