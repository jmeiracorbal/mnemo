-- mnemo:when-column-missing observations revision_count
ALTER TABLE observations ADD COLUMN revision_count INTEGER NOT NULL DEFAULT 1;
