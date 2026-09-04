-- mnemo:foreign-keys-off
DROP TRIGGER IF EXISTS obs_fts_insert;
DROP TRIGGER IF EXISTS obs_fts_update;
DROP TRIGGER IF EXISTS obs_fts_delete;
DROP TRIGGER IF EXISTS user_prompt_fts_insert;
DROP TRIGGER IF EXISTS user_prompt_fts_update;
DROP TRIGGER IF EXISTS user_prompt_fts_delete;
DROP TABLE IF EXISTS observations_fts;
DROP TABLE IF EXISTS user_prompts_fts;

ALTER TABLE agents ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE source_kinds ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tools ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE models ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mcp_clients ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE provenance_contexts ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE observation_tags ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE session_tags ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE observation_reviews ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;

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
    is_deleted INTEGER NOT NULL DEFAULT 0,
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

INSERT INTO observations_migrated (
    id, sync_id, session_id, type, title, content, tool_name,
    scope, topic_key, normalized_hash, revision_count, duplicate_count,
    last_seen_at, created_at, updated_at, is_deleted, provenance_id
)
SELECT
    id, sync_id, session_id, type, title, content, tool_name,
    scope, topic_key, normalized_hash, revision_count, duplicate_count,
    last_seen_at, created_at, updated_at,
    CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END,
    provenance_id
FROM observations;

DROP TABLE observations;
ALTER TABLE observations_migrated RENAME TO observations;

CREATE TABLE user_prompts_migrated (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    is_deleted INTEGER NOT NULL DEFAULT 0,
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

INSERT INTO user_prompts_migrated (id, sync_id, session_id, content, created_at, is_deleted, provenance_id)
SELECT id, sync_id, session_id, content, created_at, 0, provenance_id
FROM user_prompts;

DROP TABLE user_prompts;
ALTER TABLE user_prompts_migrated RENAME TO user_prompts;

CREATE TABLE sync_mutations_migrated (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    target_key TEXT NOT NULL,
    entity TEXT NOT NULL,
    entity_key TEXT NOT NULL,
    op TEXT NOT NULL,
    payload TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'local',
    occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
    acked_at TEXT,
    FOREIGN KEY (target_key) REFERENCES sync_state(target_key)
);

INSERT INTO sync_mutations_migrated (seq, target_key, entity, entity_key, op, payload, source, occurred_at, acked_at)
SELECT seq, target_key, entity, entity_key, op, payload, source, occurred_at, acked_at
FROM sync_mutations;

DROP TABLE sync_mutations;
ALTER TABLE sync_mutations_migrated RENAME TO sync_mutations;

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
CREATE INDEX IF NOT EXISTS idx_obs_is_deleted ON observations(is_deleted);
CREATE INDEX IF NOT EXISTS idx_obs_dedupe ON observations(normalized_hash, scope, type, title, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_provenance ON observations(provenance_id);
CREATE INDEX IF NOT EXISTS idx_prompts_session ON user_prompts(session_id);
CREATE INDEX IF NOT EXISTS idx_prompts_created ON user_prompts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_prompts_sync_id ON user_prompts(sync_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_prompts_sync_id ON user_prompts(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX IF NOT EXISTS idx_prompts_provenance ON user_prompts(provenance_id);
CREATE INDEX IF NOT EXISTS idx_sessions_provenance ON sessions(provenance_id);
CREATE INDEX IF NOT EXISTS idx_sync_mutations_target_seq ON sync_mutations(target_key, seq);
CREATE INDEX IF NOT EXISTS idx_sync_mutations_pending ON sync_mutations(target_key, acked_at, seq);
CREATE INDEX IF NOT EXISTS idx_review_state ON observation_reviews(state);
CREATE INDEX IF NOT EXISTS idx_review_superseded_by ON observation_reviews(superseded_by);
CREATE INDEX IF NOT EXISTS idx_obs_tags_obs ON observation_tags(observation_id);
CREATE INDEX IF NOT EXISTS idx_obs_tags_tag ON observation_tags(tag);
CREATE INDEX IF NOT EXISTS idx_ses_tags_ses ON session_tags(session_id);

CREATE TRIGGER obs_fts_insert AFTER INSERT ON observations
WHEN new.is_deleted = 0 BEGIN
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
    SELECT new.id, new.title, new.content, new.tool_name, new.type
    WHERE new.is_deleted = 0;
END;

CREATE TRIGGER user_prompt_fts_insert AFTER INSERT ON user_prompts
WHEN new.is_deleted = 0 BEGIN
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
    SELECT new.id, new.content
    WHERE new.is_deleted = 0;
END;

INSERT INTO observations_fts(rowid, title, content, tool_name, type)
SELECT id, title, content, tool_name, type FROM observations WHERE is_deleted = 0;

INSERT INTO user_prompts_fts(rowid, content)
SELECT id, content FROM user_prompts WHERE is_deleted = 0;
