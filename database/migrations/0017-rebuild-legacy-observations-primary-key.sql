-- mnemo:when-column-not-primary-key observations id
DROP TRIGGER IF EXISTS obs_fts_insert;
DROP TRIGGER IF EXISTS obs_fts_update;
DROP TRIGGER IF EXISTS obs_fts_delete;
DROP TABLE IF EXISTS observations_fts;

CREATE TABLE observations_migrated (
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
    provenance_id INTEGER REFERENCES provenance_contexts(id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

INSERT INTO observations_migrated (
    id, sync_id, session_id, type, title, content, tool_name, project,
    scope, topic_key, normalized_hash, revision_count, duplicate_count,
    last_seen_at, created_at, updated_at, deleted_at, provenance_id
)
SELECT
    CASE
        WHEN id IS NULL THEN NULL
        WHEN ROW_NUMBER() OVER (PARTITION BY id ORDER BY rowid) = 1 THEN CAST(id AS INTEGER)
        ELSE NULL
    END,
    COALESCE(NULLIF(sync_id, ''), 'obs-' || lower(hex(randomblob(16)))),
    session_id,
    COALESCE(NULLIF(type, ''), 'manual'),
    COALESCE(NULLIF(title, ''), 'Untitled observation'),
    COALESCE(content, ''),
    tool_name,
    project,
    CASE WHEN scope IS NULL OR scope = '' THEN 'project' ELSE scope END,
    NULLIF(topic_key, ''),
    normalized_hash,
    CASE WHEN revision_count IS NULL OR revision_count < 1 THEN 1 ELSE revision_count END,
    CASE WHEN duplicate_count IS NULL OR duplicate_count < 1 THEN 1 ELSE duplicate_count END,
    last_seen_at,
    COALESCE(NULLIF(created_at, ''), datetime('now')),
    COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, ''), datetime('now')),
    deleted_at,
    provenance_id
FROM observations
ORDER BY rowid;

DROP TABLE observations;
ALTER TABLE observations_migrated RENAME TO observations;
