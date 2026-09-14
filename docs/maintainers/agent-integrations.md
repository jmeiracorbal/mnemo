# Agent integrations and hooks

Read this guide before changing a hook, plugin metadata, setup flow or supported
agent integration.

## Canonical write contract

Every memory-writing command must explicitly receive:

- `project`: the unique identifier in `.mnemo`.
- `directory`: the workspace of that session, not project identity.
- `session_id`: the ID provided by the agent lifecycle event.

Do not recover values from paths, the working directory, a recent session or a
generated fallback. If the host agent cannot provide a required value, fail
clearly and fix its integration instead of adding best-effort behavior.

Claude and Codex `session_id`, Cursor `conversation_id`, and Windsurf
`trajectory_id` must be explicitly mapped to the same canonical `session_id`.
Their payload extraction may differ; their memory semantics must not.

## Hooks and plugin boundaries

The plugin owns metadata, hooks and MCP registration. The binary owns setup,
environment preparation, file modifications and protocol installation. This is
intentional; report only inconsistencies between the two surfaces.

Before changing hooks, verify names and paths across
`plugin/claude-code/hooks/hooks.json`, shipped scripts, installer code and
tests. A prior critical defect referenced `post-compaction.sh` while the actual
script was `post-compact.sh`.

Hook-internal `mnemo save`, `mnemo capture`, `mnemo session start` and
`mnemo session end` commands must suppress both streams with
`>/dev/null 2>&1`.

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
- Test that every write hook passes `project`, `directory` and `session_id`.
- Test metadata consistency and the affected hook/setup flow end to end.
- Run `claude plugin validate plugin/claude-code` when the Claude plugin changes.
- Update user documentation whenever installation, plugin or MCP behavior changes.
