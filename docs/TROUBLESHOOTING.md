# Troubleshooting

mnemo is designed to be diagnosable without mutating state.

## Health checks

After installing globally, confirm the binary is accessible:

```bash
mnemo --version
```

Run the read-only doctor:

```bash
mnemo doctor --agent=all --path=.
mnemo doctor --json --agent=codex --path=.
```

Check the compact global setup table:

```bash
mnemo setup status --agent=all
```

`setup status` is read-only:

- `Detected` means the agent's user-level configuration directory exists.
- `MCP` reports whether mnemo's MCP server is configured.
- `Hooks` reports hook/plugin runtime files, or `n/a` when that agent has no runtime surface to validate.
- `Instructions` reports whether global mnemo instructions are installed.

## Manual checks

Project activation:

```bash
cat .mnemo                          # must contain id + agents list
```

Global instructions:

```bash
grep "mnemo:start" ~/.codex/AGENTS.md ~/.claude/CLAUDE.md 2>/dev/null
head -3 ~/.cursor/rules/mnemo.mdc   # should have: alwaysApply: true
grep "mnemo:start" ~/.codeium/windsurf/memories/global_rules.md 2>/dev/null
grep "mnemo:start" ~/.config/opencode/AGENTS.md 2>/dev/null
```

Global hooks/config:

```bash
grep "mnemo" ~/.cursor/hooks.json ~/.codeium/windsurf/hooks.json ~/.codex/hooks.json 2>/dev/null
ls ~/.config/opencode/plugins/mnemo.ts ~/.config/opencode/plugins/mnemo-protocol.md
```

Canonical Agent Skill and symlinks:

```bash
test -f ~/.agents/skills/mnemo-memory/SKILL.md
ls -l ~/.claude/skills/mnemo-memory \
  ~/.codeium/windsurf/skills/mnemo-memory
```

Only symlinks for selected agent-specific consumers are expected to exist. Codex and Cursor use the canonical `.agents/skills` path directly.

## Claude Code plugin validation

```bash
claude plugin validate plugin/claude-code
```

Claude Code can also be configured by `install.sh` through MCP and global instructions without an installed plugin registry. In that case `setup status` shows `Hooks` as `n/a` and should not warn.

## Idempotency

Running setup commands repeatedly must not duplicate managed blocks or marker entries:

```bash
mnemo install-instructions --agent=codex
mnemo install-instructions --agent=codex  # second run: no duplicate block
mnemo init --agent=claudecode
mnemo init --agent=claudecode             # second run: no duplicate agent entry
```

## Common fixes

| Symptom | Check | Fix |
|---|---|---|
| Agent cannot find mnemo MCP | `mnemo --version` from the same shell/editor environment | Add `~/.local/bin` to PATH or reinstall with `MNEMO_INSTALL_DIR`. |
| Agent ignores memory | `cat .mnemo` | Run `mnemo init --agent=<agent>` in the project root. |
| Duplicate project identities | `mnemo projects list --json` | Run `mnemo projects merge --auto-by-path --dry-run` and approve only safe merges. |
| Conflicting memories | `mnemo memories review --project=<project>` | Mark stale/reviewed, supersede, or consolidate topic keys after review. |
| Setup drift | `mnemo setup status --agent=all` | Run `mnemo setup refresh --agent=all`. |
