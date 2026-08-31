-- mnemo:when-column-missing user_prompts project
ALTER TABLE user_prompts ADD COLUMN project TEXT;
