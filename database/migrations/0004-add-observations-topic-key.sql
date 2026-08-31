-- mnemo:when-column-missing observations topic_key
ALTER TABLE observations ADD COLUMN topic_key TEXT;
