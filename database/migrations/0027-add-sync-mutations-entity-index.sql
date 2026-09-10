CREATE INDEX IF NOT EXISTS idx_sync_mutations_entity_key ON sync_mutations(target_key, entity, entity_key);
