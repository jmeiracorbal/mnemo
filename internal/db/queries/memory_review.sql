-- name: ListMemoryReviewCandidates :many
SELECT o.id, o.type, o.title, ifnull(o.project, '') AS project, o.scope, ifnull(o.topic_key, '') AS topic_key,
       ifnull(o.normalized_hash, '') AS normalized_hash, o.created_at, o.updated_at,
       ifnull(r.state, '') AS review_state, ifnull(r.reason, '') AS review_reason, r.superseded_by,
       ifnull(r.reviewed_at, '') AS review_reviewed_at, ifnull(r.updated_at, '') AS review_updated_at
FROM observations o
LEFT JOIN observation_reviews r ON r.observation_id = o.id
WHERE o.deleted_at IS NULL
  AND (sqlc.arg('project') = '' OR ifnull(o.project, '') = sqlc.arg('project'))
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'))
  AND (sqlc.arg('topic_key') = '' OR ifnull(o.topic_key, '') = sqlc.arg('topic_key'))
ORDER BY ifnull(o.project, ''), o.scope, ifnull(o.topic_key, ''), lower(o.title), o.id;

-- name: UpsertMemoryReviewState :exec
INSERT INTO observation_reviews (observation_id, state, reason, superseded_by, reviewed_at, updated_at)
VALUES (
  sqlc.arg('observation_id'), sqlc.arg('state'), sqlc.arg('reason'),
  sqlc.narg('superseded_by'), sqlc.narg('reviewed_at'), datetime('now')
)
ON CONFLICT(observation_id) DO UPDATE SET
  state = excluded.state,
  reason = excluded.reason,
  superseded_by = excluded.superseded_by,
  reviewed_at = excluded.reviewed_at,
  updated_at = datetime('now');

-- name: ListObservationIDsByTopic :many
SELECT id FROM observations
WHERE deleted_at IS NULL
  AND topic_key = sqlc.arg('topic_key')
  AND (sqlc.arg('project') = '' OR ifnull(project, '') = sqlc.arg('project'))
  AND (sqlc.arg('scope') = '' OR scope = sqlc.arg('scope'))
ORDER BY id ASC;
