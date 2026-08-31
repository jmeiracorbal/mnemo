package database

import "embed"

// Schema is the canonical current SQLite schema used by sqlc and schema validation.
//
//go:embed schema.sql
var Schema string

// Migrations contains the ordered SQLite migration scripts applied at runtime.
//
//go:embed migrations/*.sql
var Migrations embed.FS
