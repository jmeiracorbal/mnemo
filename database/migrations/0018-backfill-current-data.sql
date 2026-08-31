INSERT OR IGNORE INTO agents (id, display_name, kind) VALUES
    ('unknown', 'Unknown', 'unknown'),
    ('external', 'External', 'agent'),
    ('cli', 'CLI', 'cli'),
    ('codex', 'Codex', 'agent'),
    ('claudecode', 'Claude Code', 'agent'),
    ('cursor', 'Cursor', 'agent'),
    ('windsurf', 'Windsurf', 'agent'),
    ('opencode', 'OpenCode', 'agent'),
    ('fx', 'fx', 'agent'),
    ('pi', 'Pi', 'agent');

INSERT OR IGNORE INTO source_kinds (id, display_name) VALUES
    ('unknown', 'Unknown'),
    ('cli', 'CLI'),
    ('mcp', 'MCP'),
    ('hook', 'Hook'),
    ('passive_capture', 'Passive Capture'),
    ('import', 'Import'),
    ('skill', 'Skill'),
    ('sync', 'Sync');

INSERT OR IGNORE INTO tools (id, display_name) VALUES
    ('unknown', 'Unknown'),
    ('mnemo_save', 'mnemo save'),
    ('mem_save', 'mem_save'),
    ('mem_save_prompt', 'mem_save_prompt'),
    ('mem_session_start', 'mem_session_start'),
    ('mem_session_end', 'mem_session_end'),
    ('mem_session_summary', 'mem_session_summary'),
    ('mem_capture_passive', 'mem_capture_passive'),
    ('mnemo_capture', 'mnemo capture'),
    ('mnemo_import', 'mnemo import'),
    ('sync_pull', 'sync pull'),
    ('hook_session_start', 'hook session start'),
    ('hook_session_stop', 'hook session stop');

INSERT OR IGNORE INTO models (id, provider, display_name) VALUES
    ('unknown', '', 'Unknown');

INSERT OR IGNORE INTO mcp_clients (id, name, version, transport) VALUES
    ('none', 'None', '', ''),
    ('unknown', 'Unknown', '', '');

UPDATE sync_mutations
SET project = COALESCE(json_extract(payload, '$.project'), '')
WHERE project = '' AND payload != '';

UPDATE observations SET scope = 'project' WHERE scope IS NULL OR scope = '';
UPDATE observations SET topic_key = NULL WHERE topic_key = '';
UPDATE observations SET revision_count = 1 WHERE revision_count IS NULL OR revision_count < 1;
UPDATE observations SET duplicate_count = 1 WHERE duplicate_count IS NULL OR duplicate_count < 1;
UPDATE observations SET updated_at = created_at WHERE updated_at IS NULL OR updated_at = '';
UPDATE observations SET sync_id = 'obs-' || lower(hex(randomblob(16))) WHERE sync_id IS NULL OR sync_id = '';

UPDATE user_prompts SET project = '' WHERE project IS NULL;
UPDATE user_prompts SET sync_id = 'prompt-' || lower(hex(randomblob(16))) WHERE sync_id IS NULL OR sync_id = '';
INSERT OR IGNORE INTO sync_state (target_key, lifecycle, updated_at) VALUES ('cloud', 'idle', datetime('now'));
