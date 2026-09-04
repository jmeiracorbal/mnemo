-- name: InsertPrompt :one
INSERT INTO user_prompts (sync_id, session_id, content, is_deleted, provenance_id)
VALUES (
  sqlc.arg('sync_id'), sqlc.arg('session_id'), sqlc.arg('content'),
  sqlc.arg('is_deleted'), sqlc.narg('provenance_id')
)
RETURNING id;

-- name: FindPromptBySyncID :one
SELECT id FROM user_prompts WHERE sync_id = ? ORDER BY id DESC LIMIT 1;

-- name: UpdatePrompt :exec
UPDATE user_prompts SET
  session_id = sqlc.arg('session_id'),
  content = sqlc.arg('content'),
  is_deleted = sqlc.arg('is_deleted'),
  provenance_id = COALESCE(sqlc.narg('provenance_id'), provenance_id)
WHERE id = sqlc.arg('id');
