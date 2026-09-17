CREATE TABLE processed_mcp_commands (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    result_json TEXT NOT NULL,
    processed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_processed_mcp_commands_processed_at ON processed_mcp_commands(processed_at DESC);
