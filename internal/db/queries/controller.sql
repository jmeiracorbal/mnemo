-- name: GetExecutionSessionEndedAt :one
SELECT ended_at FROM sessions
WHERE id = sqlc.arg('id') AND project = sqlc.arg('project') AND is_deleted = 0;

-- name: InsertExecutionSessionIfMissing :exec
INSERT INTO execution_sessions (project, execution_key, session_id)
VALUES (sqlc.arg('project'), sqlc.arg('execution_key'), sqlc.arg('session_id'))
ON CONFLICT(project, execution_key) DO NOTHING;

-- name: BindExecutionSessionKey :exec
INSERT INTO execution_sessions (project, execution_key, session_id)
VALUES (sqlc.arg('project'), sqlc.arg('execution_key'), sqlc.arg('session_id'))
ON CONFLICT(project, execution_key) DO UPDATE SET session_id = excluded.session_id;

-- name: InsertProcessedEvent :execrows
INSERT INTO processed_events (id, event_type, project, execution_key)
VALUES (sqlc.arg('id'), sqlc.arg('event_type'), sqlc.arg('project'), sqlc.arg('execution_key'))
ON CONFLICT(id) DO NOTHING;

-- name: GetExecutionSessionID :one
SELECT session_id FROM execution_sessions
WHERE project = sqlc.arg('project') AND execution_key = sqlc.arg('execution_key');

-- name: GetProcessedMCPCommandResult :one
SELECT result_json FROM processed_mcp_commands WHERE id = sqlc.arg('id');

-- name: InsertProcessedMCPCommand :exec
INSERT INTO processed_mcp_commands (id, action, result_json)
VALUES (sqlc.arg('id'), sqlc.arg('action'), sqlc.arg('result_json'));
