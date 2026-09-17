# Agent integrations and hooks

Read this guide before changing a hook, plugin metadata, setup flow or supported
agent integration.

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

## Hooks and plugin boundaries

The plugin owns metadata, hooks and MCP registration. The binary owns setup,
environment preparation, file modifications and protocol installation. This is
intentional; report only inconsistencies between the two surfaces.

Before changing hooks, verify names and paths across
`plugin/claude-code/hooks/hooks.json`, shipped scripts, installer code and
tests. A prior critical defect referenced `post-compaction.sh` while the actual
script was `post-compact.sh`.

Hooks and extensions may inject context and publish durable events, but must
not write SQLite or invoke retired `mnemo save`, `mnemo capture`, or `mnemo
session start/compact/end` commands. Events go to the installation-wide
controller. A hook may publish only when its adapter supplies its own native
execution ID.
The Go controller derives the canonical execution key; hooks and extensions do
not calculate it. Do not substitute a path, PID, another agent's native ID or
a latest-session lookup when it is unavailable; report the adapter capability
instead. See [`docs/DURABLE_EVENTS.md`](../DURABLE_EVENTS.md).

Shipped hooks and plugins resolve `PROJECT` exclusively from `.mnemo.id`.
They must never use a filesystem-derived identity.

## Skills

Keep the canonical skill at `~/.agents/skills/mnemo-memory/`. If an agent reads
that path directly, do not create a redundant copy or symlink. Otherwise,
declare its link through `AgentSkillSpec.GlobalLinkPath` in the agent spec; do
not add hard-coded global skill paths. Validate the resulting layout in an
isolated `HOME`.

## Required validation

- Test shipped hook configuration against real script paths and filenames.
- Test that hooks contain no direct session-associated writes.
- Test metadata consistency and the affected hook/setup flow end to end.
- Run `claude plugin validate plugin/claude-code` when the Claude plugin changes.
- Update user documentation whenever installation, plugin or MCP behavior changes.
