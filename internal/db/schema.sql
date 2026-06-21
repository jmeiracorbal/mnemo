CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project TEXT NOT NULL,
    directory TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at TEXT,
    summary TEXT
);

CREATE TABLE observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_name TEXT,
    project TEXT,
    scope TEXT NOT NULL DEFAULT 'project',
    topic_key TEXT,
    normalized_hash TEXT,
    revision_count INTEGER NOT NULL DEFAULT 1,
    duplicate_count INTEGER NOT NULL DEFAULT 1,
    last_seen_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE user_prompts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    project TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE sync_chunks (
    chunk_id TEXT PRIMARY KEY,
    imported_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sync_state (
    target_key TEXT PRIMARY KEY,
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
    project TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
    acked_at TEXT,
    FOREIGN KEY (target_key) REFERENCES sync_state(target_key)
);

CREATE TABLE sync_enrolled_projects (
    project TEXT PRIMARY KEY,
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

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE VIRTUAL TABLE observations_fts USING fts5(
    title,
    content,
    tool_name,
    type,
    project,
    content='observations',
    content_rowid='id'
);

CREATE VIRTUAL TABLE prompts_fts USING fts5(
    content,
    project,
    content='user_prompts',
    content_rowid='id'
);

CREATE INDEX idx_obs_session ON observations(session_id);
CREATE INDEX idx_obs_type ON observations(type);
CREATE INDEX idx_obs_project ON observations(project);
CREATE INDEX idx_obs_created ON observations(created_at DESC);
CREATE INDEX idx_obs_scope ON observations(scope);
CREATE INDEX idx_obs_sync_id ON observations(sync_id);
CREATE INDEX idx_obs_topic ON observations(topic_key, project, scope, updated_at DESC);
CREATE INDEX idx_obs_deleted ON observations(deleted_at);
CREATE INDEX idx_obs_dedupe ON observations(normalized_hash, project, scope, type, title, created_at DESC);
CREATE INDEX idx_prompts_session ON user_prompts(session_id);
CREATE INDEX idx_prompts_project ON user_prompts(project);
CREATE INDEX idx_prompts_created ON user_prompts(created_at DESC);
CREATE INDEX idx_prompts_sync_id ON user_prompts(sync_id);
CREATE INDEX idx_sync_mutations_target_seq ON sync_mutations(target_key, seq);
CREATE INDEX idx_sync_mutations_pending ON sync_mutations(target_key, acked_at, seq);
CREATE INDEX idx_sync_mutations_project ON sync_mutations(project);
CREATE INDEX idx_obs_tags_obs ON observation_tags(observation_id);
CREATE INDEX idx_obs_tags_tag ON observation_tags(tag);
CREATE INDEX idx_ses_tags_ses ON session_tags(session_id);

CREATE TRIGGER obs_fts_insert AFTER INSERT ON observations BEGIN
    INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
    VALUES (new.id, new.title, new.content, new.tool_name, new.type, new.project);
END;

CREATE TRIGGER obs_fts_delete AFTER DELETE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content, tool_name, type, project)
    VALUES ('delete', old.id, old.title, old.content, old.tool_name, old.type, old.project);
END;

CREATE TRIGGER obs_fts_update AFTER UPDATE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content, tool_name, type, project)
    VALUES ('delete', old.id, old.title, old.content, old.tool_name, old.type, old.project);
    INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
    VALUES (new.id, new.title, new.content, new.tool_name, new.type, new.project);
END;

CREATE TRIGGER prompt_fts_insert AFTER INSERT ON user_prompts BEGIN
    INSERT INTO prompts_fts(rowid, content, project)
    VALUES (new.id, new.content, new.project);
END;

CREATE TRIGGER prompt_fts_delete AFTER DELETE ON user_prompts BEGIN
    INSERT INTO prompts_fts(prompts_fts, rowid, content, project)
    VALUES ('delete', old.id, old.content, old.project);
END;

CREATE TRIGGER prompt_fts_update AFTER UPDATE ON user_prompts BEGIN
    INSERT INTO prompts_fts(prompts_fts, rowid, content, project)
    VALUES ('delete', old.id, old.content, old.project);
    INSERT INTO prompts_fts(rowid, content, project)
    VALUES (new.id, new.content, new.project);
END;
