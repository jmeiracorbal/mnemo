# mnemo

[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Status](https://img.shields.io/badge/status-stable-brightgreen)](https://github.com/jmeiracorbal/mnemo)
[![Storage](https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white)](https://sqlite.org)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/jmeiracorbal/mnemo)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Persistent memory for Claude Code. mnemo stores decisions, bugs, conventions, and discoveries across sessions in a local SQLite database. A one-command setup wires it into Claude Code via hooks and MCP.

---

## Features

- **Session hooks:** Automatically starts/ends sessions and injects memory context on every new conversation and `/resume`
- **14 MCP tools:** `mem_save`, `mem_search`, `mem_context`, `mem_session_summary`, and more, available directly inside Claude Code
- **Passive capture:** Extracts learnings from subagent output automatically (SubagentStop hook)
- **Full CLI:** Save, search, export, import, and inspect memories from the terminal
- **Own storage:** Isolated `~/.mnemo/memory.db`, created automatically on first run
- **One-command setup:** `mnemo setup` wires everything into Claude Code automatically

---

## Requirements

- Go 1.22+
- [Claude Code](https://claude.ai/code) CLI (`claude` command available in PATH)
- macOS or Linux (binaries available for both)

---

## Installation

### Option A: Plugin (recommended)

```bash
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo
```

> The binary must be in PATH. Download the latest release for your platform from
> [GitHub Releases](https://github.com/jmeiracorbal/mnemo/releases) and place it in `~/.local/bin/`.

### Option B: Manual build

```bash
git clone https://github.com/jmeiracorbal/mnemo
cd mnemo
go build -ldflags="-X main.version=v0.1.0" -o ~/.local/bin/mnemo ./cmd/mnemo/
```

Make sure `~/.local/bin` is in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then run setup:

```bash
mnemo setup
```

This single command:

1. Writes hook scripts to `~/.local/share/mnemo/hooks/`
2. Registers the MCP server: `claude mcp add -s user mnemo -- ~/.local/bin/mnemo mcp --tools=agent`
3. Injects hooks into `~/.claude/settings.json` (SessionStart, Stop, SubagentStop)
4. Adds all `mcp__mnemo__*` tools to `permissions.allow`
5. Writes `~/.claude/mnemo.md` (the memory protocol document)
6. Appends `@mnemo.md` to `~/.claude/CLAUDE.md`

Preview what will change without writing:

```bash
mnemo setup --dry-run
```

### Restart Claude Code

```bash
claude mcp list
# mnemo: ~/.local/bin/mnemo mcp --tools=agent  ✓ Connected
```

---

## How it works

mnemo operates through three hooks that Claude Code fires automatically:

| Hook | Trigger | Action |
|---|---|---|
| `SessionStart` | New session or `/resume` | Registers session, emits memory context |
| `Stop` | Session ends | Marks session as completed, warns if nothing was saved |
| `SubagentStop` | Subagent finishes | Passively captures learnings from subagent output |

On session start, mnemo detects the project from the git root directory name and emits relevant memories from previous sessions into the context.

---

## MCP tools

Tools available inside Claude Code via the `mcp__mnemo__*` namespace:

### Agent profile (default, 11 tools)

| Tool | Description |
|---|---|
| `mem_save` | Save a memory with title, content, type, and optional topic key |
| `mem_search` | Full-text search across all memories |
| `mem_context` | Retrieve formatted context from previous sessions |
| `mem_session_summary` | Save an end-of-session summary with goal, discoveries, next steps |
| `mem_session_start` | Register a new session |
| `mem_session_end` | Mark a session as completed |
| `mem_get_observation` | Retrieve full content of a memory by ID |
| `mem_suggest_topic_key` | Suggest a topic key for deduplication |
| `mem_capture_passive` | Extract and save learnings from free-form text |
| `mem_save_prompt` | Save a prompt template |
| `mem_update` | Update an existing memory |

### Admin profile (3 tools)

Available with `--tools=admin`: `mem_delete`, `mem_stats`, `mem_timeline`.

```bash
claude mcp add -s user mnemo-admin -- ~/.local/bin/mnemo mcp --tools=admin
```

---

## CLI reference

```
mnemo mcp [--tools=PROFILE]          Start MCP server (stdio)
mnemo save <title> <content>         Save a memory
mnemo search <query>                 Search memories
mnemo context [project]              Show context from previous sessions
mnemo session start <id>             Register session start
mnemo session end <id>               Mark session as completed
mnemo session exists <id>            Check if a session exists (exits 1 if not)
mnemo session obs-count <id>         Count observations saved in a session
mnemo stats                          Show memory statistics
mnemo export [file]                  Export all memories to JSON
mnemo import <file.json>             Import memories from JSON
mnemo capture <content>              Extract learnings from text (passive capture)
mnemo setup [--dry-run]              Install hooks and configure Claude Code
mnemo version                        Show version
```

### Examples

```bash
# Save a decision manually
mnemo save "Use FTS5 for search" "Chose SQLite FTS5 over external search" --type decision --project myapp

# Search memories
mnemo search "authentication" --project myapp --limit 5

# Show what was remembered from previous sessions
mnemo context myapp

# Export everything to JSON
mnemo export backup.json
```

---

## Storage

mnemo uses `~/.mnemo/memory.db`, created automatically on first run. The directory and database are created during startup, no manual setup required. The schema uses SQLite with FTS5 for full-text search.

---

## License

[Apache 2.0](LICENSE): You may use, modify, and distribute freely, but must retain the copyright notice and include the [NOTICE](NOTICE) file in all distributions.
