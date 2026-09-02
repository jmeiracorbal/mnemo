DROP TRIGGER IF EXISTS user_prompt_fts_insert;
DROP TRIGGER IF EXISTS user_prompt_fts_update;
DROP TRIGGER IF EXISTS user_prompt_fts_delete;
DROP TRIGGER IF EXISTS prompt_fts_insert;
DROP TRIGGER IF EXISTS prompt_fts_update;
DROP TRIGGER IF EXISTS prompt_fts_delete;
DROP TABLE IF EXISTS user_prompts_fts;
DROP TABLE IF EXISTS prompts_fts;

CREATE VIRTUAL TABLE user_prompts_fts USING fts5(
    content,
    project,
    content='user_prompts',
    content_rowid='id'
);

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

INSERT INTO user_prompts_fts(rowid, content, project)
SELECT id, content, project FROM user_prompts;
