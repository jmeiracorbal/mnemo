-- mnemo:when-column-missing user_prompts provenance_id
ALTER TABLE user_prompts ADD COLUMN provenance_id INTEGER REFERENCES provenance_contexts(id);
