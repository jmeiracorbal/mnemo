-- mnemo:foreign-keys-off
DROP TRIGGER IF EXISTS obs_fts_insert;
DROP TRIGGER IF EXISTS obs_fts_update;
DROP TRIGGER IF EXISTS obs_fts_delete;
DROP TRIGGER IF EXISTS user_prompt_fts_insert;
DROP TRIGGER IF EXISTS user_prompt_fts_update;
DROP TRIGGER IF EXISTS user_prompt_fts_delete;
DROP TRIGGER IF EXISTS prompt_fts_insert;
DROP TRIGGER IF EXISTS prompt_fts_update;
DROP TRIGGER IF EXISTS prompt_fts_delete;
DROP TABLE IF EXISTS observations_fts;
DROP TABLE IF EXISTS user_prompts_fts;
DROP TABLE IF EXISTS prompts_fts;

INSERT OR IGNORE INTO projects (id, name)
SELECT project, project FROM sessions WHERE project IS NOT NULL AND project != ''
UNION
SELECT project, project FROM observations WHERE project IS NOT NULL AND project != ''
UNION
SELECT project, project FROM user_prompts WHERE project IS NOT NULL AND project != ''
UNION
SELECT project, project FROM sync_mutations WHERE project IS NOT NULL AND project != ''
UNION
SELECT project, project FROM sync_enrolled_projects WHERE project IS NOT NULL AND project != '';

CREATE TABLE sessions_migrated (
    id TEXT PRIMARY KEY,
    project TEXT NOT NULL REFERENCES projects(id),
    directory TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at TEXT,
    summary TEXT,
    provenance_id INTEGER REFERENCES provenance_contexts(id)
);

INSERT INTO sessions_migrated (id, project, directory, started_at, ended_at, summary, provenance_id)
SELECT id, project, directory, started_at, ended_at, summary, provenance_id
FROM sessions;

DROP TABLE sessions;
ALTER TABLE sessions_migrated RENAME TO sessions;

CREATE TABLE observations_migrated (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_name TEXT,
    project TEXT REFERENCES projects(id),
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
    id, sync_id, session_id, type, title, content, tool_name, project,
    scope, topic_key, normalized_hash, revision_count, duplicate_count,
    last_seen_at, created_at, updated_at, deleted_at, provenance_id
)
SELECT
    id, sync_id, session_id, type, title, content, tool_name, NULLIF(project, ''),
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
    project TEXT REFERENCES projects(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

INSERT INTO user_prompts_migrated (id, sync_id, session_id, content, project, created_at, provenance_id)
SELECT id, sync_id, session_id, content, NULLIF(project, ''), created_at, provenance_id
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
    project TEXT REFERENCES projects(id),
    occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
    acked_at TEXT,
    FOREIGN KEY (target_key) REFERENCES sync_state(target_key)
);

INSERT INTO sync_mutations_migrated (seq, target_key, entity, entity_key, op, payload, source, project, occurred_at, acked_at)
SELECT seq, target_key, entity, entity_key, op, payload, source, NULLIF(project, ''), occurred_at, acked_at
FROM sync_mutations;

DROP TABLE sync_mutations;
ALTER TABLE sync_mutations_migrated RENAME TO sync_mutations;

CREATE TABLE sync_enrolled_projects_migrated (
    project TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    enrolled_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO sync_enrolled_projects_migrated (project, enrolled_at)
SELECT project, enrolled_at FROM sync_enrolled_projects WHERE project IS NOT NULL AND project != '';

DROP TABLE sync_enrolled_projects;
ALTER TABLE sync_enrolled_projects_migrated RENAME TO sync_enrolled_projects;

CREATE VIRTUAL TABLE observations_fts USING fts5(
    title,
    content,
    tool_name,
    type,
    project,
    content='observations',
    content_rowid='id'
);

CREATE VIRTUAL TABLE user_prompts_fts USING fts5(
    content,
    project,
    content='user_prompts',
    content_rowid='id'
);

CREATE INDEX IF NOT EXISTS idx_obs_session ON observations(session_id);
CREATE INDEX IF NOT EXISTS idx_obs_type ON observations(type);
CREATE INDEX IF NOT EXISTS idx_obs_project ON observations(project);
CREATE INDEX IF NOT EXISTS idx_obs_created ON observations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_scope ON observations(scope);
CREATE INDEX IF NOT EXISTS idx_obs_sync_id ON observations(sync_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_observations_sync_id ON observations(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX IF NOT EXISTS idx_obs_topic ON observations(topic_key, project, scope, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_deleted ON observations(deleted_at);
CREATE INDEX IF NOT EXISTS idx_obs_dedupe ON observations(normalized_hash, project, scope, type, title, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_provenance ON observations(provenance_id);
CREATE INDEX IF NOT EXISTS idx_prompts_session ON user_prompts(session_id);
CREATE INDEX IF NOT EXISTS idx_prompts_project ON user_prompts(project);
CREATE INDEX IF NOT EXISTS idx_prompts_created ON user_prompts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_prompts_sync_id ON user_prompts(sync_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_prompts_sync_id ON user_prompts(sync_id) WHERE sync_id IS NOT NULL AND sync_id <> '';
CREATE INDEX IF NOT EXISTS idx_prompts_provenance ON user_prompts(provenance_id);
CREATE INDEX IF NOT EXISTS idx_sessions_provenance ON sessions(provenance_id);
CREATE INDEX IF NOT EXISTS idx_sync_mutations_target_seq ON sync_mutations(target_key, seq);
CREATE INDEX IF NOT EXISTS idx_sync_mutations_pending ON sync_mutations(target_key, acked_at, seq);
CREATE INDEX IF NOT EXISTS idx_sync_mutations_project ON sync_mutations(project);

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

CREATE TRIGGER user_prompt_fts_insert AFTER INSERT ON user_prompts BEGIN
    INSERT INTO user_prompts_fts(rowid, content, project)
    VALUES (new.id, new.content, new.project);
END;

CREATE TRIGGER user_prompt_fts_delete AFTER DELETE ON user_prompts BEGIN
    INSERT INTO user_prompts_fts(user_prompts_fts, rowid, content, project)
    VALUES ('delete', old.id, old.content, old.project);
END;

CREATE TRIGGER user_prompt_fts_update AFTER UPDATE ON user_prompts BEGIN
    INSERT INTO user_prompts_fts(user_prompts_fts, rowid, content, project)
    VALUES ('delete', old.id, old.content, old.project);
    INSERT INTO user_prompts_fts(rowid, content, project)
    VALUES (new.id, new.content, new.project);
END;

INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
SELECT id, title, content, tool_name, type, project FROM observations WHERE deleted_at IS NULL;

INSERT INTO user_prompts_fts(rowid, content, project)
SELECT id, content, project FROM user_prompts;
