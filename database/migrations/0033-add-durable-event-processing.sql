CREATE TABLE execution_sessions (
    project TEXT NOT NULL REFERENCES projects(id),
    execution_key TEXT NOT NULL,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project, execution_key),
    UNIQUE (session_id)
);

CREATE TABLE processed_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    project TEXT NOT NULL REFERENCES projects(id),
    execution_key TEXT NOT NULL,
    processed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_execution_sessions_session ON execution_sessions(session_id);
CREATE INDEX idx_processed_events_project ON processed_events(project, processed_at DESC);
