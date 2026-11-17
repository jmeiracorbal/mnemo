# Project invariants

Mnemo ships one Go binary. The binary owns every agent integration: `mnemo
setup refresh`, called by `install.sh` and `mnemo update`, installs global
instructions, MCP registration, hooks and skills. `mnemo doctor --fix` repairs
those integrations. Do not add a second registration path.

## Core invariants

- Project identity is only the `id` in `.mnemo`; never derive it from a path.
- Every memory-writing MCP caller explicitly passes canonical `project` and
  session-workspace `directory`. The MCP runtime creates and owns the session;
  callers never pass, infer or generate a session identifier.
- Equivalent events preserve identical memory semantics across supported
  agents. Hooks may inject context, but must not create, close or write
  sessions.
- Schema changes require a new incremental migration and an updated
  `database/target_schema.sql`; never edit an existing migration.
- The binary version is the single consistency boundary. The MCP server
  reports the build-injected binary version, never a separate constant.

## Required maintainer guides

| Before changing… | Read first |
|---|---|
| hooks, setup, or an agent integration | [`agent-integrations.md`](agent-integrations.md) |
| versions, installer/update behavior, or a release | [`releases.md`](releases.md) |
| database schema or sqlc queries | [`migrations.md`](migrations.md) |

## Verification

For code changes, run `go test ./...`, `go build ./...`, and `git diff --check`.
For a changed hook or setup flow, also follow the relevant guide's validation
checklist. Report the exact files, user impact, minimal fix, and verification
evidence; distinguish bugs, documentation issues, and design decisions.
