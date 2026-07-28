-- name: EnsureProject :exec
INSERT OR IGNORE INTO projects (id, name) VALUES (?, ?);

-- name: GetProjectByID :one
SELECT id, name, created_at FROM projects WHERE id = ?;

-- name: ListProjects :many
SELECT id, name, created_at FROM projects ORDER BY created_at ASC;


-- name: ListProjectSummaries :many
WITH project_ids AS (
    SELECT id AS project FROM projects
    UNION
    SELECT project FROM sessions WHERE project != ''
    UNION
    SELECT project FROM observations WHERE project IS NOT NULL AND project != '' AND deleted_at IS NULL
    UNION
    SELECT project FROM user_prompts WHERE project IS NOT NULL AND project != ''
), activity AS (
    SELECT project, started_at AS seen_at FROM sessions WHERE project != ''
    UNION ALL
    SELECT project, COALESCE(NULLIF(last_seen_at, ''), NULLIF(updated_at, ''), created_at) AS seen_at
    FROM observations
    WHERE project IS NOT NULL AND project != '' AND deleted_at IS NULL
    UNION ALL
    SELECT project, created_at AS seen_at FROM user_prompts WHERE project IS NOT NULL AND project != ''
)
SELECT
    p.project AS id,
    COALESCE(NULLIF(pr.name, ''), p.project) AS name,
    COALESCE(pr.created_at, '') AS created_at,
    CAST(COALESCE((SELECT s.directory FROM sessions s WHERE s.project = p.project AND s.directory != '' ORDER BY s.started_at DESC LIMIT 1), '') AS TEXT) AS directory,
    (SELECT COUNT(*) FROM sessions s WHERE s.project = p.project) AS session_count,
    (SELECT COUNT(*) FROM observations o WHERE o.project = p.project AND o.deleted_at IS NULL) AS observation_count,
    (SELECT COUNT(*) FROM user_prompts up WHERE up.project = p.project) AS prompt_count,
    CAST(COALESCE(MAX(activity.seen_at), '') AS TEXT) AS last_seen_at
FROM project_ids p
LEFT JOIN projects pr ON pr.id = p.project
LEFT JOIN activity ON activity.project = p.project
GROUP BY p.project, pr.name, pr.created_at
ORDER BY p.project ASC;
