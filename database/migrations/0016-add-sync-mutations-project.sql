-- mnemo:when-column-missing sync_mutations project
ALTER TABLE sync_mutations ADD COLUMN project TEXT NOT NULL DEFAULT '';
