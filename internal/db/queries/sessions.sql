-- name: UpsertSession :exec
INSERT INTO sessions (id, project, directory)
VALUES (?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  project = CASE WHEN sessions.project = '' THEN excluded.project ELSE sessions.project END,
  directory = CASE WHEN sessions.directory = '' THEN excluded.directory ELSE sessions.directory END;

-- name: EndSession :exec
UPDATE sessions
SET ended_at = datetime('now'), summary = sqlc.narg('summary')
WHERE id = sqlc.arg('id');

-- name: GetSession :one
SELECT id, project, directory, started_at, ended_at, summary
FROM sessions WHERE id = ?;

-- name: CountSessionObservations :one
SELECT COUNT(*) FROM observations
WHERE session_id = ? AND deleted_at IS NULL;

-- name: CountSessionObservationsByScope :one
SELECT COUNT(*)
FROM observations o
WHERE o.session_id = sqlc.arg('session_id')
  AND o.deleted_at IS NULL
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'));

-- name: CountObservationsForSessionProject :one
SELECT COUNT(*)
FROM observations o
JOIN sessions ss ON ss.id = sqlc.arg('session_id')
WHERE o.project = ss.project
  AND o.created_at >= ss.started_at
  AND o.deleted_at IS NULL;

-- name: ListSessions :many
SELECT s.id, s.project, s.started_at, s.ended_at, s.summary,
       COUNT(o.id) AS observation_count
FROM sessions s
LEFT JOIN observations o ON o.session_id = s.id AND o.deleted_at IS NULL
WHERE (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
GROUP BY s.id
ORDER BY MAX(COALESCE(o.created_at, s.started_at)) DESC
LIMIT sqlc.arg('result_limit');

-- name: GetSessionPayload :one
SELECT project, directory, ended_at, summary FROM sessions WHERE id = ?;

-- name: ListSessionTags :many
SELECT tag FROM session_tags WHERE session_id = ? ORDER BY tag;

-- name: DeleteSessionTags :exec
DELETE FROM session_tags WHERE session_id = ?;

-- name: InsertSessionTag :exec
INSERT OR IGNORE INTO session_tags (session_id, tag) VALUES (?, ?);
