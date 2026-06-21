-- name: GetObservation :one
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, type, title, content,
       tool_name, project, scope, topic_key, revision_count, duplicate_count,
       last_seen_at, created_at, updated_at, deleted_at
FROM observations WHERE id = ?;

-- name: GetLiveObservation :one
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, type, title, content,
       tool_name, project, scope, topic_key, revision_count, duplicate_count,
       last_seen_at, created_at, updated_at, deleted_at
FROM observations WHERE id = ? AND deleted_at IS NULL;

-- name: GetLiveObservationBySyncID :one
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, type, title, content,
       tool_name, project, scope, topic_key, revision_count, duplicate_count,
       last_seen_at, created_at, updated_at, deleted_at
FROM observations WHERE sync_id = ? AND deleted_at IS NULL
ORDER BY id DESC LIMIT 1;

-- name: GetObservationBySyncIDIncludingDeleted :one
SELECT id, ifnull(sync_id, '') AS sync_id, session_id, type, title, content,
       tool_name, project, scope, topic_key, revision_count, duplicate_count,
       last_seen_at, created_at, updated_at, deleted_at
FROM observations WHERE sync_id = ?
ORDER BY id DESC LIMIT 1;

-- name: FindObservationByTopic :one
SELECT id FROM observations
WHERE topic_key = sqlc.arg('topic_key')
  AND ifnull(project, '') = ifnull(sqlc.narg('project'), '')
  AND scope = sqlc.arg('scope')
  AND deleted_at IS NULL
ORDER BY datetime(updated_at) DESC, datetime(created_at) DESC
LIMIT 1;

-- name: FindDuplicateObservation :one
SELECT id FROM observations
WHERE normalized_hash = sqlc.arg('normalized_hash')
  AND ifnull(project, '') = ifnull(sqlc.narg('project'), '')
  AND scope = sqlc.arg('scope')
  AND type = sqlc.arg('type')
  AND title = sqlc.arg('title')
  AND deleted_at IS NULL
  AND datetime(created_at) >= datetime('now', sqlc.arg('window'))
ORDER BY id DESC LIMIT 1;

-- name: InsertObservation :one
INSERT INTO observations (
  sync_id, session_id, type, title, content, tool_name, project, scope,
  topic_key, normalized_hash, revision_count, duplicate_count, last_seen_at, updated_at
) VALUES (
  sqlc.arg('sync_id'), sqlc.arg('session_id'), sqlc.arg('type'), sqlc.arg('title'),
  sqlc.arg('content'), sqlc.narg('tool_name'), sqlc.narg('project'), sqlc.arg('scope'),
  sqlc.narg('topic_key'), sqlc.arg('normalized_hash'), 1, 1, datetime('now'), datetime('now')
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
  revision_count = revision_count + 1,
  last_seen_at = datetime('now'),
  updated_at = datetime('now')
WHERE id = sqlc.arg('id');

-- name: UpdateObservationFields :exec
UPDATE observations SET
  type = sqlc.arg('type'),
  title = sqlc.arg('title'),
  content = sqlc.arg('content'),
  project = sqlc.narg('project'),
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
SELECT id FROM observations
WHERE normalized_hash = sqlc.narg('normalized_hash')
  AND ifnull(project, '') = ifnull(sqlc.narg('project'), '')
  AND deleted_at IS NULL
LIMIT 1;
