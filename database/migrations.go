package database

import "embed"

// Schema is the target SQLite schema — the complete expected state after all
// migrations have been applied. It is used by ValidateCurrent to verify
// post-migration consistency and by tests to confirm that running all
// migrations produces an equivalent result. It must be updated alongside
// every new migration.
//
//go:embed target_schema.sql
var Schema string

// Migrations contains the ordered SQLite migration scripts applied at runtime.
//
//go:embed migrations/*.sql
var Migrations embed.FS
