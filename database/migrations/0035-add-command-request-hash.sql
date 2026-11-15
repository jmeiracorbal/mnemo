-- mnemo:when-column-missing processed_mcp_commands request_hash
ALTER TABLE processed_mcp_commands ADD COLUMN request_hash TEXT NOT NULL DEFAULT '';
