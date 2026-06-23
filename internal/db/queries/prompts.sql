-- name: InsertPrompt :one
INSERT INTO user_prompts (sync_id, session_id, content, project)
VALUES (sqlc.arg('sync_id'), sqlc.arg('session_id'), sqlc.arg('content'), sqlc.narg('project'))
RETURNING id;

-- name: FindPromptBySyncID :one
SELECT id FROM user_prompts WHERE sync_id = ? ORDER BY id DESC LIMIT 1;

-- name: UpdatePrompt :exec
UPDATE user_prompts SET session_id = ?, content = ?, project = ? WHERE id = ?;

