-- mnemo:foreign-keys-off
CREATE TABLE sync_types (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL UNIQUE
);

INSERT OR IGNORE INTO sync_types (id, display_name)
VALUES ('cloud', 'Cloud synchronization');

ALTER TABLE sync_state
ADD COLUMN sync_type_id TEXT NOT NULL DEFAULT 'cloud'
REFERENCES sync_types(id);
