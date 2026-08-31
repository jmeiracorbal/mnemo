-- mnemo:when-column-missing observations last_seen_at
ALTER TABLE observations ADD COLUMN last_seen_at TEXT;
