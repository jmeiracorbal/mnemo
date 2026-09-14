# AGENTS.md

## Purpose

This repository ships the `mnemo` Go binary and the Claude Code plugin. They
are deliberately separate: the plugin provides metadata, hooks and MCP
integration; the binary performs setup and file changes that the plugin cannot.

## Core invariants

- Project identity is only the `id` in `.mnemo`; never derive it from a path.
- Every MCP memory-writing caller must explicitly pass canonical `project` and
  session-workspace `directory`. The MCP runtime creates and owns its session;
  callers never pass, infer or generate a session identifier.
- Equivalent events must preserve identical memory semantics for every supported
  agent. Hooks may inject context, but must not create, close or write sessions.
- Schema changes require a new incremental migration and an updated
  `database/target_schema.sql`; never edit an existing migration.
- Binary and plugin version metadata are a single consistency boundary. The MCP
  server reports the build-injected binary version, never a separate constant.

## Required reading by change area

| Before changing… | Read first |
|---|---|
| hooks, plugin setup, or an agent integration | [`docs/maintainers/agent-integrations.md`](docs/maintainers/agent-integrations.md) |
| versions, installer/update behavior, or a release | [`docs/maintainers/releases.md`](docs/maintainers/releases.md) |
| database schema or sqlc queries | [`docs/maintainers/migrations.md`](docs/maintainers/migrations.md) |

Do not make an area-specific change until its required guide has been read.

## Verification and reporting

Run `go test ./...`, `go build ./...`, and `git diff --check` for every code
change. For a changed hook/plugin/setup flow, also follow the relevant guide's
validation checklist. State exact files, user impact, minimal fix, and evidence
of verification; distinguish bugs, documentation issues and design decisions.

<!-- mnemo:start -->
## mnemo

You have access to mnemo MCP tools: mem_save, mem_search, mem_context, mem_session_summary.

### MEMORY AUTHORITY

mnemo is the ONLY persistent memory system for this project.
NEVER use native agent memory, `MEMORY.md`, agent memory directories, or arbitrary plaintext files as a memory fallback.
When asked to remember or save something, always use `mem_save`.
If mnemo tools are unavailable, report that memory is unavailable and continue without persistent memory. Do not create an alternative memory store.

Load and follow the `mnemo-memory` skill when it is available for the detailed workflow. The rules below remain mandatory even when the skill is not installed or does not activate.

### PROACTIVE SAVE

Call `mem_save` immediately after any of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented or updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

Self-check after every task: "Did I just make a decision, fix a bug, learn something, or establish a convention? If yes, call mem_save now."

### SEARCH MEMORY

Search when:
- User asks to recall anything
- Starting work on something that might have been done before
- User mentions a topic you have no context on

Use `mem_context` first for broad recent context, then `mem_search` for focused recall.

### SUBAGENT OUTPUT

When running as a subagent, end your response with:

```markdown
## Key Learnings
- <learning 1>
- <learning 2>
```

Omit only if the task produced no learnings worth retaining.

### SESSION CLOSE

`mem_session_summary` is not optional. It is the final step of every session.
Call it before any response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without `mem_session_summary`.
<!-- mnemo:end -->
