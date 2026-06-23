-- name: EnsureProject :exec
INSERT OR IGNORE INTO projects (id, name) VALUES (?, ?);

-- name: GetProjectByID :one
SELECT id, name, created_at FROM projects WHERE id = ?;

-- name: ListProjects :many
SELECT id, name, created_at FROM projects ORDER BY created_at ASC;

