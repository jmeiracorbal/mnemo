ALTER TABLE sessions ADD COLUMN mcp_instance_id TEXT;
CREATE UNIQUE INDEX ux_sessions_open_mcp_instance
ON sessions(project, mcp_instance_id)
WHERE mcp_instance_id IS NOT NULL AND ended_at IS NULL AND is_deleted = 0;
