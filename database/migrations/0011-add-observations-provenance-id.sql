-- mnemo:when-column-missing observations provenance_id
ALTER TABLE observations ADD COLUMN provenance_id INTEGER REFERENCES provenance_contexts(id);
