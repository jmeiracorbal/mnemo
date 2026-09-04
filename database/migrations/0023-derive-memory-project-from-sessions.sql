-- mnemo:foreign-keys-off
DROP TRIGGER IF EXISTS obs_fts_insert;
DROP TRIGGER IF EXISTS obs_fts_update;
DROP TRIGGER IF EXISTS obs_fts_delete;
DROP TRIGGER IF EXISTS user_prompt_fts_insert;
DROP TRIGGER IF EXISTS user_prompt_fts_update;
DROP TRIGGER IF EXISTS user_prompt_fts_delete;
DROP TABLE IF EXISTS observations_fts;
DROP TABLE IF EXISTS user_prompts_fts;

CREATE TABLE observations_migrated (
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

INSERT INTO observations_migrated (
    id, sync_id, session_id, type, title, content, tool_name,
    scope, topic_key, normalized_hash, revision_count, duplicate_count,
    last_seen_at, created_at, updated_at, deleted_at, provenance_id
)
SELECT
    id, sync_id, session_id, type, title, content, tool_name,
    scope, topic_key, normalized_hash, revision_count, duplicate_count,
    last_seen_at, created_at, updated_at, deleted_at, provenance_id
FROM observations;

DROP TABLE observations;
ALTER TABLE observations_migrated RENAME TO observations;

CREATE TABLE user_prompts_migrated (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

INSERT INTO user_prompts_migrated (id, sync_id, session_id, content, created_at, provenance_id)
SELECT id, sync_id, session_id, content, created_at, provenance_id
FROM user_prompts;

DROP TABLE user_prompts;
ALTER TABLE user_prompts_migrated RENAME TO user_prompts;

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

CREATE INDEX IF NOT EXISTS idx_obs_session ON observations(session_id);
CREATE INDEX IF NOT EXISTS idx_obs_type ON observations(type);
CREATE INDEX IF NOT EXISTS idx_obs_created ON observations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_scope ON observations(scope);
CREATE INDEX IF NOT EXISTS idx_obs_sync_id ON observations(sync_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_observations_sync_id ON observations(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX IF NOT EXISTS idx_obs_topic ON observations(topic_key, scope, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_deleted ON observations(deleted_at);
CREATE INDEX IF NOT EXISTS idx_obs_dedupe ON observations(normalized_hash, scope, type, title, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_provenance ON observations(provenance_id);
CREATE INDEX IF NOT EXISTS idx_prompts_session ON user_prompts(session_id);
CREATE INDEX IF NOT EXISTS idx_prompts_created ON user_prompts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_prompts_sync_id ON user_prompts(sync_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_prompts_sync_id ON user_prompts(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX IF NOT EXISTS idx_prompts_provenance ON user_prompts(provenance_id);

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

INSERT INTO observations_fts(rowid, title, content, tool_name, type)
SELECT id, title, content, tool_name, type FROM observations WHERE deleted_at IS NULL;

INSERT INTO user_prompts_fts(rowid, content)
SELECT id, content FROM user_prompts;
