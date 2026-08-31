-- mnemo:when-column-missing observations updated_at
ALTER TABLE observations ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
