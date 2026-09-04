-- name: UpsertSession :exec
INSERT INTO sessions (id, project, directory, provenance_id)
VALUES (?, ?, ?, sqlc.narg('provenance_id'))
ON CONFLICT(id) DO UPDATE SET
  project = CASE WHEN sessions.project = '' THEN excluded.project ELSE sessions.project END,
  directory = CASE WHEN sessions.directory = '' THEN excluded.directory ELSE sessions.directory END,
  is_deleted = 0,
  provenance_id = COALESCE(sessions.provenance_id, excluded.provenance_id);

-- name: EndSession :exec
UPDATE sessions
SET ended_at = datetime('now'), summary = sqlc.narg('summary')
WHERE id = sqlc.arg('id');

-- name: GetSession :one
SELECT id, project, directory, started_at, ended_at, summary, provenance_id
FROM sessions WHERE id = ? AND is_deleted = 0;

-- name: CountSessionObservations :one
SELECT COUNT(*) FROM observations
WHERE session_id = ? AND is_deleted = 0;

-- name: CountSessionObservationsByScope :one
SELECT COUNT(*)
FROM observations o
WHERE o.session_id = sqlc.arg('session_id')
  AND o.is_deleted = 0
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'));

-- name: CountObservationsForSessionProject :one
SELECT COUNT(*)
FROM observations o
JOIN sessions os ON os.id = o.session_id
JOIN sessions ss ON ss.id = sqlc.arg('session_id')
WHERE ss.is_deleted = 0
  AND os.is_deleted = 0
  AND os.project = ss.project
  AND o.created_at >= ss.started_at
  AND o.is_deleted = 0;

-- name: ListSessions :many
SELECT s.id, s.project, s.started_at, s.ended_at, s.summary, s.provenance_id,
       COUNT(o.id) AS observation_count
FROM sessions s
LEFT JOIN observations o ON o.session_id = s.id AND o.is_deleted = 0
WHERE (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND s.is_deleted = 0
GROUP BY s.id
ORDER BY MAX(COALESCE(o.created_at, s.started_at)) DESC
LIMIT sqlc.arg('result_limit');

-- name: GetSessionPayload :one
SELECT project, directory, ended_at, summary, is_deleted, provenance_id FROM sessions WHERE id = ?;

-- name: ListSessionTags :many
SELECT tag FROM session_tags WHERE session_id = ? AND is_deleted = 0 ORDER BY tag;

-- name: DeleteSessionTags :exec
UPDATE session_tags SET is_deleted = 1 WHERE session_id = ?;

-- name: InsertSessionTag :exec
INSERT INTO session_tags (session_id, tag, is_deleted) VALUES (?, ?, 0)
ON CONFLICT(session_id, tag) DO UPDATE SET is_deleted = 0;
