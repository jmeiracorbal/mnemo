-- name: EnsureProject :exec
INSERT INTO projects (id, name, is_deleted) VALUES (?, ?, 0)
ON CONFLICT(id) DO UPDATE SET is_deleted = 0;

-- name: UpsertProjectName :exec
INSERT INTO projects (id, name, is_deleted) VALUES (?, ?, 0)
ON CONFLICT(id) DO UPDATE SET name = excluded.name, is_deleted = 0;

-- name: GetProjectByID :one
SELECT id, name, is_deleted, created_at FROM projects WHERE id = ? AND is_deleted = 0;

-- name: CountProjectRows :one
SELECT COUNT(*) FROM projects WHERE id = ?;

-- name: DeleteProjectByID :execrows
UPDATE projects SET is_deleted = 1 WHERE id = ?;

-- name: ListProjects :many
SELECT id, name, is_deleted, created_at FROM projects WHERE is_deleted = 0 ORDER BY created_at ASC;


-- name: ListProjectSummaries :many
WITH project_ids AS (
    SELECT id AS project FROM projects WHERE is_deleted = 0
), activity AS (
    SELECT project, started_at AS seen_at FROM sessions WHERE project != '' AND is_deleted = 0
    UNION ALL
    SELECT s.project, COALESCE(NULLIF(o.last_seen_at, ''), NULLIF(o.updated_at, ''), o.created_at) AS seen_at
    FROM observations o
    JOIN sessions s ON s.id = o.session_id
    WHERE o.is_deleted = 0
    UNION ALL
    SELECT s.project, up.created_at AS seen_at
    FROM user_prompts up
    JOIN sessions s ON s.id = up.session_id
    WHERE up.is_deleted = 0 AND s.is_deleted = 0
)
SELECT
    p.project AS id,
    COALESCE(NULLIF(pr.name, ''), p.project) AS name,
    COALESCE(pr.created_at, '') AS created_at,
    CAST(COALESCE((SELECT s.directory FROM sessions s WHERE s.project = p.project AND s.directory != '' ORDER BY s.started_at DESC LIMIT 1), '') AS TEXT) AS directory,
    (SELECT COUNT(*) FROM sessions s WHERE s.project = p.project AND s.is_deleted = 0) AS session_count,
    (SELECT COUNT(*) FROM observations o JOIN sessions s ON s.id = o.session_id WHERE s.project = p.project AND s.is_deleted = 0 AND o.is_deleted = 0) AS observation_count,
    (SELECT COUNT(*) FROM user_prompts up JOIN sessions s ON s.id = up.session_id WHERE s.project = p.project AND s.is_deleted = 0 AND up.is_deleted = 0) AS prompt_count,
    CAST(COALESCE(MAX(activity.seen_at), '') AS TEXT) AS last_seen_at
FROM project_ids p
JOIN projects pr ON pr.id = p.project
LEFT JOIN activity ON activity.project = p.project
GROUP BY p.project, pr.name, pr.created_at
ORDER BY p.project ASC;
