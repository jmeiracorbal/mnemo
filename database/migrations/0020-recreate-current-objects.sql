DROP TRIGGER IF EXISTS obs_fts_insert;
DROP TRIGGER IF EXISTS obs_fts_update;
DROP TRIGGER IF EXISTS obs_fts_delete;
DROP TRIGGER IF EXISTS prompt_fts_insert;
DROP TRIGGER IF EXISTS prompt_fts_update;
DROP TRIGGER IF EXISTS prompt_fts_delete;
DROP TABLE IF EXISTS observations_fts;
DROP TABLE IF EXISTS prompts_fts;

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

CREATE INDEX IF NOT EXISTS idx_provenance_agent ON provenance_contexts(agent_id);

CREATE INDEX IF NOT EXISTS idx_provenance_source ON provenance_contexts(source_kind_id);

CREATE INDEX IF NOT EXISTS idx_provenance_tool ON provenance_contexts(tool_id);

CREATE INDEX IF NOT EXISTS idx_provenance_model ON provenance_contexts(model_id);

CREATE INDEX IF NOT EXISTS idx_provenance_mcp_client ON provenance_contexts(mcp_client_id);

CREATE INDEX IF NOT EXISTS idx_sync_mutations_target_seq ON sync_mutations(target_key, seq);

CREATE INDEX IF NOT EXISTS idx_sync_mutations_pending ON sync_mutations(target_key, acked_at, seq);

CREATE INDEX IF NOT EXISTS idx_sync_mutations_project ON sync_mutations(project);

CREATE INDEX IF NOT EXISTS idx_review_state ON observation_reviews(state);

CREATE INDEX IF NOT EXISTS idx_review_superseded_by ON observation_reviews(superseded_by);

CREATE INDEX IF NOT EXISTS idx_obs_tags_obs ON observation_tags(observation_id);

CREATE INDEX IF NOT EXISTS idx_obs_tags_tag ON observation_tags(tag);

CREATE INDEX IF NOT EXISTS idx_ses_tags_ses ON session_tags(session_id);

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

INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
SELECT id, title, content, tool_name, type, project FROM observations WHERE deleted_at IS NULL;

INSERT INTO prompts_fts(rowid, content, project)
SELECT id, content, project FROM user_prompts;
