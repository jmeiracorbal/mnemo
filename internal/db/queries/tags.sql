-- name: ListTagAggregates :many
SELECT ot.tag, COUNT(*) AS count, MAX(datetime(o.created_at)) AS last_used_at
FROM observation_tags ot
JOIN observations o ON o.id = ot.observation_id
JOIN sessions s ON s.id = o.session_id
WHERE ot.is_deleted = 0
  AND o.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
GROUP BY ot.tag
ORDER BY count DESC, ot.tag ASC;

-- name: ListRelatedObservationTags :many
SELECT ot2.tag, COUNT(*) AS count, MAX(o.created_at) AS last_seen_at
FROM observation_tags ot1
JOIN observation_tags ot2
  ON ot1.observation_id = ot2.observation_id AND ot2.tag != ot1.tag
JOIN observations o ON o.id = ot1.observation_id
JOIN sessions s ON s.id = o.session_id
WHERE ot1.tag = sqlc.arg('tag')
  AND ot1.is_deleted = 0
  AND ot2.is_deleted = 0
  AND o.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('since') = '' OR o.created_at >= datetime(sqlc.arg('since')))
GROUP BY ot2.tag;

-- name: ListRelatedSessionTags :many
SELECT st2.tag, COUNT(*) AS count, MAX(s.started_at) AS last_seen_at
FROM session_tags st1
JOIN session_tags st2
  ON st1.session_id = st2.session_id AND st2.tag != st1.tag
JOIN sessions s ON s.id = st1.session_id
WHERE st1.tag = sqlc.arg('tag')
  AND st1.is_deleted = 0
  AND st2.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('since') = '' OR s.started_at >= datetime(sqlc.arg('since')))
GROUP BY st2.tag;

-- name: ListObservationsAffectedByTag :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title,
       o.content, o.tool_name, s.project AS project, o.scope, o.topic_key, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
JOIN observation_tags ot ON ot.observation_id = o.id
WHERE ot.tag = ? AND ot.is_deleted = 0 AND o.is_deleted = 0;

-- name: ListSessionsAffectedByTag :many
SELECT s.id, s.project, s.directory, s.ended_at, s.summary, s.provenance_id
FROM sessions s
JOIN session_tags st ON st.session_id = s.id
WHERE st.tag = ? AND st.is_deleted = 0;

-- name: CopyObservationTag :exec
INSERT INTO observation_tags (observation_id, tag, is_deleted)
SELECT observation_id, sqlc.arg('to_tag'), 0
FROM observation_tags
WHERE observation_tags.tag = sqlc.arg('from_tag') AND observation_tags.is_deleted = 0
ON CONFLICT(observation_id, tag) DO UPDATE SET is_deleted = 0;

-- name: DeleteObservationTagByName :exec
UPDATE observation_tags SET is_deleted = 1 WHERE tag = ?;

-- name: CopySessionTag :exec
INSERT INTO session_tags (session_id, tag, is_deleted)
SELECT session_id, sqlc.arg('to_tag'), 0
FROM session_tags
WHERE session_tags.tag = sqlc.arg('from_tag') AND session_tags.is_deleted = 0
ON CONFLICT(session_id, tag) DO UPDATE SET is_deleted = 0;

-- name: DeleteSessionTagByName :exec
UPDATE session_tags SET is_deleted = 1 WHERE tag = ?;
