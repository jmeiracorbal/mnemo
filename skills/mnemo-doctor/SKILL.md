---
name: mnemo-doctor
description: Diagnose and repair the mnemo installation. ALWAYS use when a mnemo MCP tool (mem_*) is missing, duplicated or failing, when a mnemo hook reports an error, when `mnemo doctor` reports warnings or errors, or after installing or upgrading mnemo. Repairs go through `mnemo doctor --fix`; never edit agent configuration by hand.
---

# mnemo Doctor

mnemo configures each agent from the installed binary: global instructions,
the MCP server, hooks and skills. `mnemo doctor` checks that configuration and
`mnemo doctor --fix` restores it. This skill is independent from
`mnemo-memory`; use `mnemo-memory` for normal context recovery and saves.

## When to use

- A `mem_*` tool is unavailable, appears twice, or fails with a configuration
  or controller error.
- A mnemo hook prints an error, or session context from mnemo is missing.
- After installing or upgrading mnemo, or when the user asks whether mnemo is
  working.

## Workflow

1. Run `mnemo doctor` from the project directory. Add `--agent=<agent>` to
   limit the report to the current agent (`claudecode`, `codex`, `cursor`,
   `opencode`, `pi`).
2. Read every `warning` and `error`. Each one prints a `fix:` line with the
   command that repairs it.
3. Run `mnemo doctor --fix` (with the same `--agent` if you used it). It
   removes legacy mnemo plugins through the agent's own CLI and rewrites the
   mnemo configuration for the affected agents. It is safe to run repeatedly.
4. Run `mnemo doctor` again and report the final status to the user.
5. If the MCP server or hooks changed, tell the user to restart the agent
   session so it loads them.

## Rules

- Never edit `.claude.json`, `settings.json`, `config.toml`, `hooks.json` or
  plugin registries by hand to make mnemo work. Use `mnemo doctor --fix` or
  `mnemo setup refresh`.
- Never create an alternative memory store while mnemo is broken. Report that
  persistent memory is unavailable and continue without it.
- If `--fix` cannot finish (for example, the agent CLI it needs is not on
  `PATH`), show the user the exact command from the `fix:` line instead of
  improvising another repair.
- A missing `.mnemo` marker is not an installation problem: the project is not
  initialized. Suggest `mnemo init` only when the user wants mnemo in it.
