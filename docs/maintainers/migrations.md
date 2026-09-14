# Database migrations and sqlc

Read this guide before changing database schema or SQL queries.

## Schema contract

The schema has two synchronized representations:

- `database/migrations/NNNN-<name>.sql`: ordered runtime upgrades.
- `database/target_schema.sql`: the complete final schema used by validation and
  migration tests.

`target_schema.sql` is not an initial schema. Fresh installations apply all
migrations in sequence. Never modify an existing migration.

## Required procedure

1. Add a new numbered migration for every schema change.
2. Update `database/target_schema.sql` to the resulting final state.
3. Update the expected latest version in
   `internal/db/migrate/migrate_test.go`.
4. Regenerate sqlc output when a query changes.
5. Run `go test ./internal/db/migrate/...` and confirm canonical-schema
   validation passes.

If the target schema is not updated, `ValidateCurrent` and migrated databases
will diverge.
