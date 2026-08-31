-- mnemo:when-column-missing user_prompts sync_id
ALTER TABLE user_prompts ADD COLUMN sync_id TEXT;
