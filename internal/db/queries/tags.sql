-- name: ListTagAggregates :many
SELECT ot.tag, COUNT(*) AS count, MAX(datetime(o.created_at)) AS last_used_at
FROM observation_tags ot
JOIN observations o ON o.id = ot.observation_id
WHERE o.deleted_at IS NULL
  AND (sqlc.arg('project') = '' OR o.project = sqlc.arg('project'))
GROUP BY ot.tag
ORDER BY count DESC, ot.tag ASC;

-- name: ListRelatedObservationTags :many
SELECT ot2.tag, COUNT(*) AS count, MAX(o.created_at) AS last_seen_at
FROM observation_tags ot1
JOIN observation_tags ot2
  ON ot1.observation_id = ot2.observation_id AND ot2.tag != ot1.tag
JOIN observations o ON o.id = ot1.observation_id
WHERE ot1.tag = sqlc.arg('tag')
  AND o.deleted_at IS NULL
  AND (sqlc.arg('project') = '' OR o.project = sqlc.arg('project'))
  AND (sqlc.arg('since') = '' OR o.created_at >= datetime(sqlc.arg('since')))
GROUP BY ot2.tag;

-- name: ListRelatedSessionTags :many
SELECT st2.tag, COUNT(*) AS count, MAX(s.started_at) AS last_seen_at
FROM session_tags st1
JOIN session_tags st2
  ON st1.session_id = st2.session_id AND st2.tag != st1.tag
JOIN sessions s ON s.id = st1.session_id
WHERE st1.tag = sqlc.arg('tag')
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('since') = '' OR s.started_at >= datetime(sqlc.arg('since')))
GROUP BY st2.tag;

-- name: ListObservationsAffectedByTag :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title,
       o.content, o.tool_name, o.project, o.scope, o.topic_key
FROM observations o
JOIN observation_tags ot ON ot.observation_id = o.id
WHERE ot.tag = ? AND o.deleted_at IS NULL;

-- name: ListSessionsAffectedByTag :many
SELECT s.id, s.project, s.directory, s.ended_at, s.summary
FROM sessions s
JOIN session_tags st ON st.session_id = s.id
WHERE st.tag = ?;

-- name: CopyObservationTag :exec
INSERT OR IGNORE INTO observation_tags (observation_id, tag)
SELECT observation_id, sqlc.arg('to_tag')
FROM observation_tags
WHERE observation_tags.tag = sqlc.arg('from_tag');

-- name: DeleteObservationTagByName :exec
DELETE FROM observation_tags WHERE tag = ?;

-- name: CopySessionTag :exec
INSERT OR IGNORE INTO session_tags (session_id, tag)
SELECT session_id, sqlc.arg('to_tag')
FROM session_tags
WHERE session_tags.tag = sqlc.arg('from_tag');

-- name: DeleteSessionTagByName :exec
DELETE FROM session_tags WHERE tag = ?;
