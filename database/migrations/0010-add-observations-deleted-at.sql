-- mnemo:when-column-missing observations deleted_at
ALTER TABLE observations ADD COLUMN deleted_at TEXT;
