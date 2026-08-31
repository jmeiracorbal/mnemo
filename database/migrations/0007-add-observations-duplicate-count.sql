-- mnemo:when-column-missing observations duplicate_count
ALTER TABLE observations ADD COLUMN duplicate_count INTEGER NOT NULL DEFAULT 1;
