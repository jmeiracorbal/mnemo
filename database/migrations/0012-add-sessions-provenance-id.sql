-- mnemo:when-column-missing sessions provenance_id
ALTER TABLE sessions ADD COLUMN provenance_id INTEGER REFERENCES provenance_contexts(id);
