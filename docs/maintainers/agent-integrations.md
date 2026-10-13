# Agent integrations and hooks

Read this guide before changing a hook, setup flow or supported agent
integration.

## Canonical MCP write contract

Every memory-writing MCP call must explicitly receive:

- `project`: the unique identifier in `.mnemo`.
- `directory`: the workspace of that session, not project identity.

The controller creates and owns the session bound to the adapter's native
execution ID. Callers must not provide, recover or generate mnemo session IDs.
MCP request metadata, inherited process environment, or an adapter-defined
hook-written side channel supplies the native ID. The runtime rejects a missing
or malformed documented carrier rather than substituting its PID, path or a
recent session.

Every supported agent therefore has the same session semantics. The controller
derives the canonical execution key in Go from adapter + project + native ID;
no hook, extension or MCP client submits a precomputed key.

## Native extension transports

MCP is not mandatory when an agent provides a stronger native extension API.
Pi is the current example: `scripts/pi/extensions/mnemo.ts` obtains
`ctx.sessionManager.getSessionId()` for every lifecycle event and custom-tool
call. Its tools invoke the controller with `agent=pi` and that native ID; the
extension never receives or submits a mnemo session ID. Do not restore Pi's
static MCP configuration: it cannot inject a Pi session ID into its child
process. Register tools synchronously while loading the extension, not from an
asynchronous session event.

## Integration ownership

The `mnemo` binary is the only owner of every agent integration. `mnemo setup
refresh` installs global instructions, the MCP server registration, hooks and
skills; `install.sh` and `mnemo update` run it, and `mnemo doctor --fix` runs it
to repair an agent. Do not ship agent plugins or marketplace entries: earlier
releases did, Claude Code and Codex then started two mnemo MCP servers, and the
plugin and setup drifted apart.

Hook scripts are embedded assets under `scripts/<agent>/hooks/`. Setup installs
them with explicit file modes and registers them with absolute paths in the
agent's own configuration:

- Claude Code: scripts in `<config dir>/hooks/mnemo/`, registered in
  `<config dir>/settings.json` in exec form (`command` plus `args`), where
  `<config dir>` is `CLAUDE_CONFIG_DIR` or `~/.claude`. Every hook whose command
  lives in that directory belongs to mnemo; setup replaces those entries and
  leaves the user's other hooks and settings untouched.
- Codex and Cursor: scripts and `hooks.json` under `~/.codex` and `~/.cursor`.

Before changing hooks, verify names and paths across the embedded scripts, the
registration in the agent spec, the `doctor` runtime check and tests.

Earlier releases installed a `mnemo@mnemo` plugin for Claude Code and Codex.
`mnemo doctor` reports it as `legacy_plugin.<agent>` and `mnemo doctor --fix`
removes it through the agent's own CLI (`claude plugin uninstall`, `codex
plugin remove`); setup never edits an agent's plugin registry directly.

Hooks and extensions may inject context and publish durable events, but must
not write SQLite or invoke retired `mnemo save`, `mnemo capture`, or `mnemo
session start/compact/end` commands. Events go to the installation-wide
controller. A hook may publish only when its adapter supplies its own native
execution ID.
The Go controller derives the canonical execution key; hooks and extensions do
not calculate it. Do not substitute a path, PID, another agent's native ID or
a latest-session lookup when it is unavailable; report the adapter capability
instead. See [`docs/DURABLE_EVENTS.md`](../DURABLE_EVENTS.md).

Shipped hooks and extensions resolve `PROJECT` exclusively from `.mnemo.id`.
They must never use a filesystem-derived identity.

## Skills

Setup installs every embedded skill (`mnemo-memory` and `mnemo-doctor`) under
`~/.agents/skills/<name>/`. If an agent reads that path directly, do not create
a redundant copy or symlink. Otherwise, declare the agent's skills directory
through `AgentSkillSpec.SkillsDir` in the agent spec; setup links each skill
into it. Do not add hard-coded global skill paths. Validate the resulting
layout in an isolated `HOME`.

## Required validation

- Test shipped hook configuration against real script paths and filenames.
- Test that hooks contain no direct session-associated writes.
- Test metadata consistency and the affected hook/setup flow end to end.
- Test `mnemo doctor` and `mnemo doctor --fix` for the affected agent.
- Update user documentation whenever installation, hook or MCP behavior changes.
