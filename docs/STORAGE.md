# Storage

mnemo stores data locally in SQLite.

## Location

```text
~/.mnemo/memory.db
```

The database is created automatically on first use. Project activation markers live in each project as `.mnemo`, but the memory store itself is user-local.

## Schema and search

The schema uses SQLite with FTS5 for full-text search. Runtime queries are defined under `internal/db/queries` and compiled into type-safe Go code with sqlc.

Important concepts:

| Concept | Purpose |
|---|---|
| `sessions` | Tracks agent sessions and their project/directory context. |
| `observations` | Stores durable memories: decisions, bug fixes, discoveries, conventions and notes. |
| `user_prompts` | Stores prompt templates or user prompt records when captured. |
| `observation_reviews` | Tracks reviewed/stale/superseded memory conflict states. |
| FTS tables | Provide local full-text search. |

## Development workflow

After changing the schema or a query, regenerate and test:

```bash
go tool sqlc generate
git diff --exit-code -- internal/db/generated
go test ./...
```

For normal validation in this repository, use writable Go caches when the default module cache is not writable:

```bash
GOCACHE=/private/tmp/mnemo-go-build-cache-go1266 \
GOMODCACHE=/private/tmp/mnemo-go-mod-cache-go1266 \
go test ./...
```
