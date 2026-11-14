# Database migrations and sqlc

Read this guide before changing database schema or SQL queries.

## Schema contract

The schema has two synchronized representations:

- `database/migrations/NNNN-<name>.sql`: ordered forward upgrades.
- `database/migrations/NNNN-<name>.down.sql`: optional, explicitly reviewed
  inverse of a reversible operation; it does not change the forward checksum.
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

## Targeted schema recovery

`mnemo db migrate --target=VERSION` requires the controller to be offline.
The command applies missing forward migrations only through the selected version,
or executes explicit down scripts in reverse order. All required down scripts
are validated before a downgrade begins. Each individual migration and its
`schema_migrations` bookkeeping commit in one SQLite transaction. Unknown versions
and destructive migrations without a down script are rejected without guessing.

The first audited reverse operation is `0032-drop-mcp-pid-index.down.sql`. Its
forward migration removes only an index, so reinstating the original index is
lossless. Do not add down scripts for table or column removal that discards user
data. For those changes the release installer restores a verified pre-upgrade
SQLite recovery point instead.

## Release-to-schema compatibility contract

The executable's embedded, immutable numbered migrations define its maximum
supported schema version and checksums. Opening a database with an unknown
future migration, mismatched checksum, or dirty history fails closed; the
controller never silently downgrades the database. The installer uses a
verified SQLite snapshot to restore the exact prior schema and data before
reactivating the previous executable. A legacy JetStream delivery failure
cannot trigger a binary-only downgrade after commit.

The release journal is authoritative for rollback and commit. There is no
secondary automatic executable-only recovery path or persistent
`schema-changed` marker.

The release transaction holds a durable `~/.mnemo/upgrade.transaction` journal,
blocks writes via `~/.mnemo/upgrade.maintenance`, and keeps the previous
executable until schema, data, and controller verification have succeeded.
When an interrupted transaction is recovered, the prior binary and snapshot
are restored before the write fence is released. Once the journal is committed,
recovery never replays an old snapshot over newly acknowledged memories.
The three newest recovery points are retained; a point referenced by the current
transaction is protected. Incomplete pending recovery copies are removed only
when the transaction is settled.
