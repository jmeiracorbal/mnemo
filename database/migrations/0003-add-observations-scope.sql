-- mnemo:when-column-missing observations scope
ALTER TABLE observations ADD COLUMN scope TEXT NOT NULL DEFAULT 'project';
