# Documentation index

This directory contains the extended mnemo documentation. Start here when you need setup and update details, command references, agent integration notes, storage internals, troubleshooting guidance, or research material for future memory migration features.

## Core documentation

| Document | Purpose |
|---|---|
| [Installation](INSTALLATION.md) | Install script, self-update flow, plugin setup, project activation, and verification. |
| [Agent integration](AGENT_INTEGRATION.md) | Hook behavior, global paths, `.mnemo` marker semantics, and Agent Skills integration. |
| [CLI reference](CLI.md) | Commands, examples, MCP tools, and search modes. |
| [Troubleshooting](TROUBLESHOOTING.md) | `doctor`, `setup status`, manual checks, and idempotency validation. |
| [Storage](STORAGE.md) | SQLite location, schema notes, migrations, and sqlc workflow. |

## Research and design notes

| Area | Purpose |
|---|---|
| [Agent memory surfaces](memories/README.md) | Cross-agent memory equivalence model and importer design implications. |
| [Claude Code memory surfaces](memories/claudecode.md) | Claude Code `CLAUDE.md`, rules, and auto-memory index behavior. |
| [Codex memory surfaces](memories/codex.md) | Codex `AGENTS.md` discovery and conversion notes. |
| [Cursor memory surfaces](memories/cursor.md) | Cursor rules, User Rules, Team Rules, and import candidates. |
| [Windsurf memory surfaces](memories/windsurf.md) | Cascade Memories, Rules, and conversion notes. |
| [OpenCode memory surfaces](memories/opencode.md) | OpenCode instruction discovery and conversion notes. |
| [Pi memory surfaces](memories/pi.md) | Pi context files, system prompt modes, Skills, and optional MCP extension behavior. |
| [fx memory surfaces](memories/fx.md) | fx `AGENTS.md` instructions and `~/.fx/memories.json` memory store. |
