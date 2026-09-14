ALTER TABLE sessions ADD COLUMN mcp_pid INTEGER;
CREATE INDEX idx_sessions_mcp_pid ON sessions(project, mcp_pid) WHERE mcp_pid IS NOT NULL;
