-- mnemo:when-column-missing observations sync_id
ALTER TABLE observations ADD COLUMN sync_id TEXT;
