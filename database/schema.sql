CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    dirty INTEGER NOT NULL DEFAULT 0,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'agent',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE source_kinds (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tools (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE models (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE mcp_clients (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '',
    transport TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE provenance_contexts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL DEFAULT 'unknown' REFERENCES agents(id),
    source_kind_id TEXT NOT NULL DEFAULT 'unknown' REFERENCES source_kinds(id),
    tool_id TEXT NOT NULL DEFAULT 'unknown' REFERENCES tools(id),
    model_id TEXT NOT NULL DEFAULT 'unknown' REFERENCES models(id),
    mcp_client_id TEXT NOT NULL DEFAULT 'none' REFERENCES mcp_clients(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(agent_id, source_kind_id, tool_id, model_id, mcp_client_id)
);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project TEXT NOT NULL REFERENCES projects(id),
    directory TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at TEXT,
    summary TEXT,
    provenance_id INTEGER REFERENCES provenance_contexts(id)
);

CREATE TABLE observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_name TEXT,
    scope TEXT NOT NULL DEFAULT 'project',
    topic_key TEXT,
    normalized_hash TEXT,
    revision_count INTEGER NOT NULL DEFAULT 1,
    duplicate_count INTEGER NOT NULL DEFAULT 1,
    last_seen_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at TEXT,
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE user_prompts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE sync_types (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL UNIQUE
);

CREATE TABLE sync_chunks (
    chunk_id TEXT PRIMARY KEY,
    imported_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sync_state (
    target_key TEXT PRIMARY KEY,
    sync_type_id TEXT NOT NULL REFERENCES sync_types(id),
    lifecycle TEXT NOT NULL DEFAULT 'idle',
    last_enqueued_seq INTEGER NOT NULL DEFAULT 0,
    last_acked_seq INTEGER NOT NULL DEFAULT 0,
    last_pulled_seq INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    backoff_until TEXT,
    lease_owner TEXT,
    lease_until TEXT,
    last_error TEXT,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sync_mutations (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    target_key TEXT NOT NULL,
    entity TEXT NOT NULL,
    entity_key TEXT NOT NULL,
    op TEXT NOT NULL,
    payload TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'local',
    project TEXT REFERENCES projects(id),
    occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
    acked_at TEXT,
    FOREIGN KEY (target_key) REFERENCES sync_state(target_key)
);

CREATE TABLE sync_enrolled_projects (
    project TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    enrolled_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE observation_tags (
    observation_id INTEGER NOT NULL REFERENCES observations(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (observation_id, tag)
);

CREATE TABLE session_tags (
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (session_id, tag)
);

CREATE TABLE observation_reviews (
    observation_id INTEGER PRIMARY KEY REFERENCES observations(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    superseded_by INTEGER REFERENCES observations(id),
    reviewed_at TEXT,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE VIRTUAL TABLE observations_fts USING fts5(
    title,
    content,
    tool_name,
    type,
    content='observations',
    content_rowid='id'
);

CREATE VIRTUAL TABLE user_prompts_fts USING fts5(
    content,
    content='user_prompts',
    content_rowid='id'
);

CREATE INDEX idx_obs_session ON observations(session_id);
CREATE INDEX idx_obs_type ON observations(type);
CREATE INDEX idx_obs_created ON observations(created_at DESC);
CREATE INDEX idx_obs_scope ON observations(scope);
CREATE INDEX idx_obs_sync_id ON observations(sync_id);
CREATE UNIQUE INDEX ux_observations_sync_id ON observations(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX idx_obs_topic ON observations(topic_key, scope, updated_at DESC);
CREATE INDEX idx_obs_deleted ON observations(deleted_at);
CREATE INDEX idx_obs_dedupe ON observations(normalized_hash, scope, type, title, created_at DESC);
CREATE INDEX idx_obs_provenance ON observations(provenance_id);
CREATE INDEX idx_prompts_session ON user_prompts(session_id);
CREATE INDEX idx_prompts_created ON user_prompts(created_at DESC);
CREATE INDEX idx_prompts_sync_id ON user_prompts(sync_id);
CREATE UNIQUE INDEX ux_user_prompts_sync_id ON user_prompts(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX idx_prompts_provenance ON user_prompts(provenance_id);
CREATE INDEX idx_sessions_provenance ON sessions(provenance_id);
CREATE INDEX idx_provenance_agent ON provenance_contexts(agent_id);
CREATE INDEX idx_provenance_source ON provenance_contexts(source_kind_id);
CREATE INDEX idx_provenance_tool ON provenance_contexts(tool_id);
CREATE INDEX idx_provenance_model ON provenance_contexts(model_id);
CREATE INDEX idx_provenance_mcp_client ON provenance_contexts(mcp_client_id);
CREATE INDEX idx_sync_mutations_target_seq ON sync_mutations(target_key, seq);
CREATE INDEX idx_sync_mutations_pending ON sync_mutations(target_key, acked_at, seq);
CREATE INDEX idx_sync_mutations_project ON sync_mutations(project);
CREATE INDEX idx_review_state ON observation_reviews(state);
CREATE INDEX idx_review_superseded_by ON observation_reviews(superseded_by);
CREATE INDEX idx_obs_tags_obs ON observation_tags(observation_id);
CREATE INDEX idx_obs_tags_tag ON observation_tags(tag);
CREATE INDEX idx_ses_tags_ses ON session_tags(session_id);

CREATE TRIGGER obs_fts_insert AFTER INSERT ON observations BEGIN
    INSERT INTO observations_fts(rowid, title, content, tool_name, type)
    VALUES (new.id, new.title, new.content, new.tool_name, new.type);
END;

CREATE TRIGGER obs_fts_delete AFTER DELETE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content, tool_name, type)
    VALUES ('delete', old.id, old.title, old.content, old.tool_name, old.type);
END;

CREATE TRIGGER obs_fts_update AFTER UPDATE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content, tool_name, type)
    VALUES ('delete', old.id, old.title, old.content, old.tool_name, old.type);
    INSERT INTO observations_fts(rowid, title, content, tool_name, type)
    VALUES (new.id, new.title, new.content, new.tool_name, new.type);
END;

CREATE TRIGGER user_prompt_fts_insert AFTER INSERT ON user_prompts BEGIN
    INSERT INTO user_prompts_fts(rowid, content)
    VALUES (new.id, new.content);
END;

CREATE TRIGGER user_prompt_fts_delete AFTER DELETE ON user_prompts BEGIN
    INSERT INTO user_prompts_fts(user_prompts_fts, rowid, content)
    VALUES ('delete', old.id, old.content);
END;

CREATE TRIGGER user_prompt_fts_update AFTER UPDATE ON user_prompts BEGIN
    INSERT INTO user_prompts_fts(user_prompts_fts, rowid, content)
    VALUES ('delete', old.id, old.content);
    INSERT INTO user_prompts_fts(rowid, content)
    VALUES (new.id, new.content);
END;
