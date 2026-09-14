-- name: ProjectExists :one
SELECT EXISTS(
  SELECT 1 FROM projects pr WHERE pr.id = sqlc.arg('project_name') AND pr.is_deleted = 0
  UNION SELECT 1 FROM sessions s WHERE s.project = sqlc.arg('project_name') AND s.is_deleted = 0
);

-- name: CountObservationProjectRows :one
SELECT COUNT(*)
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE s.project = sqlc.arg('project_name');

-- name: CountSessionProjectRows :one
SELECT COUNT(*) FROM sessions WHERE project = sqlc.arg('project_name');

-- name: CountPromptProjectRows :one
SELECT COUNT(*)
FROM user_prompts p
JOIN sessions s ON s.id = p.session_id
WHERE s.project = sqlc.arg('project_name');

-- name: RenameSessionProject :execrows
UPDATE sessions SET project = sqlc.arg('new_name') WHERE project = sqlc.arg('old_name');
