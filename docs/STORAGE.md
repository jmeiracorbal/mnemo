# Storage

mnemo stores data locally in SQLite.

## Location

```text
~/.mnemo/memory.db
```

The database is created automatically on first use. Project activation markers live in each project as `.mnemo`, but the memory store itself is user-local.

## Schema and search

The schema uses SQLite with FTS5 for full-text search. The canonical current schema lives in `database/schema.sql`, versioned runtime migrations live in `database/migrations/`, and runtime queries are defined under `internal/db/queries` and compiled into type-safe Go code with sqlc.

Important concepts:

| Concept | Purpose |
|---|---|
| `sessions` | Tracks agent sessions and their project/directory context. |
| `observations` | Stores durable memories: decisions, bug fixes, discoveries, conventions and notes. |
| `user_prompts` | Stores prompt templates or user prompt records when captured. |
| `observation_reviews` | Tracks reviewed/stale/superseded memory conflict states. |
| FTS tables | Provide local full-text search; FTS table names follow their owning domain tables, e.g. `observations_fts` and `user_prompts_fts`. |

## Canonical synchronization

The canonical domain tables are the source of truth for synchronization. Each
row in a synchronizable table produces an independent `sync_mutations` entry;
relationships are synchronized by their own table rows rather than by nested
aggregate payloads. Soft deletion is represented by `is_deleted = 1` and is
replicated as a normal upsert, never as a physical delete.

`sync_state`, `sync_types`, and `sync_mutations` are local synchronization
metadata. FTS tables and their shadow tables are derived indexes and are not
replicated. If the queue is lost or needs to be reset, it can be rebuilt from
all canonical rows, including soft-deleted rows.

## Database migrations

mnemo applies safe pending migrations automatically when the store opens. If a database is inconsistent, dirty, or ahead of the bundled migrations, mnemo blocks with a clear error instead of trying an unsafe repair. Use the explicit database command for CI, troubleshooting, or manual repair checks:

```bash
mnemo db migrate --check
mnemo db migrate
mnemo db migrate --json
```

`mnemo doctor` also runs the same read-only schema validator and reports whether the local store is missing, pending, current, or inconsistent. Migration `0021` renames the prompt search index from the legacy `prompts_fts` name to `user_prompts_fts` and recreates the FTS triggers/shadow-table prefix without preserving the old objects.

Released mnemo binaries also check for newer releases on interactive CLI use.
When one exists, mnemo asks before installing. Users can run `mnemo update`
directly to review, confirm, download and install, or `mnemo update --check
--json` for automation. The update installs the mnemo binary and refreshes
mnemo's agent integration files; it does not update the agent applications
themselves. The check/prompt is skipped for MCP, hooks, and JSON-output commands
so agent integrations stay machine-readable.

## Development workflow

After changing the schema, a migration, or a query, regenerate and test:

```bash
go tool sqlc generate
git diff --exit-code -- internal/db/generated
go test ./internal/db/migrate ./internal/store ./internal/doctor ./cmd/mnemo
go test ./...
```

For normal validation in this repository, use writable Go caches when the default module cache is not writable:

```bash
GOCACHE=/private/tmp/mnemo-go-build-cache-go1266 \
GOMODCACHE=/private/tmp/mnemo-go-mod-cache-go1266 \
go test ./...
```
