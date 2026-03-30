# mnemo

[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Status](https://img.shields.io/badge/status-stable-brightgreen)](https://github.com/jmeiracorbal/mnemo)
[![Storage](https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white)](https://sqlite.org)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-blueviolet?logo=anthropic&logoColor=white)](https://claude.ai/code)
[![Cursor](https://img.shields.io/badge/Cursor-2.6%2B-000000?logo=cursor&logoColor=white)](https://cursor.com)
[![Windsurf](https://img.shields.io/badge/Windsurf-supported-0066CC)](https://codeium.com/windsurf)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/jmeiracorbal/mnemo)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Persistent memory for AI coding agents. mnemo stores decisions, bugs, conventions, and discoveries across sessions in a local SQLite database. A one-command setup wires it into Claude Code, Cursor, or Windsurf via hooks and MCP.

---

## Prerequisite: binary in PATH

**The `mnemo` binary must be installed and accessible in your `PATH` before any agent integration will work.** This applies to all agents — Claude Code, Cursor, and Windsurf — regardless of how the integration is installed (plugin or setup command).

The hooks that fire on session start, session end, and passive capture all call `mnemo` directly. The MCP server is also the `mnemo` binary. Without it in PATH, hooks fail silently and the MCP server cannot start.

Install the binary first:

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/jmeiracorbal/mnemo
cd mnemo
go build -o ~/.local/bin/mnemo ./cmd/mnemo/
export PATH="$HOME/.local/bin:$PATH"
```

Verify:

```bash
mnemo --version
```

Only after this step proceed with agent setup.

---

## Features

- **Session hooks:** Automatically starts/ends sessions and injects memory context at the beginning of every conversation
- **14 MCP tools:** `mem_save`, `mem_search`, `mem_context`, `mem_session_summary`, and more, available directly inside your editor
- **Passive capture:** Extracts learnings from conversation transcripts automatically at session end
- **Full CLI:** Save, search, export, import, and inspect memories from the terminal
- **Own storage:** Isolated `~/.mnemo/memory.db`, created automatically on first run
- **Claude Code + Cursor + Windsurf:** Native integration for all three editors via their respective hook systems
---

## Agent setup

### Claude Code

Two installation paths:

**Via plugin (recommended):**

```bash
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo
```

The plugin registers the MCP server, hooks (SessionStart, Stop, SubagentStop, PostCompact), and writes the memory protocol to `~/.claude/mnemo.md` and `~/.claude/CLAUDE.md` on first session start.

**Via setup command:**

```bash
mnemo setup
```

This command:

1. Writes hook scripts to `~/.local/share/mnemo/hooks/`
2. Registers the MCP server via `claude mcp add -s user mnemo`
3. Injects hooks into `~/.claude/settings.json`
4. Adds all `mcp__mnemo__*` tools to `permissions.allow`
5. Writes `~/.claude/mnemo.md` (memory protocol)
6. Appends `@mnemo.md` to `~/.claude/CLAUDE.md`

```bash
mnemo setup --dry-run   # preview without changes
```

### Cursor

```bash
mnemo setup --cursor
```

This command:

1. Writes hook scripts to `~/.local/share/mnemo/cursor-hooks/`
2. Registers the MCP server in `~/.cursor/mcp.json`
3. Adds hooks to `~/.cursor/hooks.json` (beforeSubmitPrompt, stop)
4. Writes `~/.cursor/rules/mnemo.mdc` (memory protocol, `alwaysApply: true`)

```bash
mnemo setup --cursor --dry-run   # preview without changes
```

Restart Cursor after setup.

### Windsurf

```bash
mnemo setup --windsurf
```

This command:

1. Writes hook scripts to `~/.local/share/mnemo/windsurf-hooks/`
2. Registers the MCP server in `~/.codeium/windsurf/mcp_config.json`
3. Adds hooks to `~/.codeium/windsurf/hooks.json` (pre_user_prompt, post_cascade_response_with_transcript)
4. Appends the memory protocol to `~/.codeium/windsurf/memories/global_rules.md`

```bash
mnemo setup --windsurf --dry-run   # preview without changes
```

Restart Windsurf after setup.

---

## Verification checklist

Run this checklist after every installation or update. All points must pass before considering the setup complete.

```bash
# 1. Binary accessible
mnemo --version

# 2. Dry-runs show expected output (no errors)
mnemo setup --dry-run
mnemo setup --cursor --dry-run
mnemo setup --windsurf --dry-run

# 3. Plugin validation (Claude Code plugin path)
claude plugin validate plugin/claude-code

# 4. Protocol files exist with correct content
cat ~/.claude/CLAUDE.md                          # must contain: @mnemo.md
head -1 ~/.claude/mnemo.md                      # must be: ## mnemo — Persistent Memory Protocol
head -3 ~/.cursor/rules/mnemo.mdc               # must have: alwaysApply: true
grep "mnemo — Persistent Memory Protocol" \
  ~/.codeium/windsurf/memories/global_rules.md  # must match

# 5. Idempotency: running setup again must report "already up to date"
mnemo setup --cursor
mnemo setup --windsurf
```

---

## How it works

### Claude Code

| Hook | Trigger | Action |
|---|---|---|
| `SessionStart` (startup/resume/clear) | New session | Registers session, injects memory context |
| `SessionStart` (compact) | After compaction | Recovers context from mnemo after context window reset |
| `PostCompact` | During compaction | Persists compaction summary to mnemo |
| `Stop` | Session ends | Marks session completed, warns if nothing was saved |
| `SubagentStop` | Subagent finishes | Passively captures learnings from subagent output |

### Cursor

| Hook | Trigger | Action |
|---|---|---|
| `beforeSubmitPrompt` | First prompt of a conversation | Registers session, injects memory context |
| `stop` | Conversation ends | Reads transcript JSONL for passive capture, closes session |

### Windsurf

| Hook | Trigger | Action |
|---|---|---|
| `pre_user_prompt` | First prompt of a conversation | Registers session, injects memory context |
| `post_cascade_response_with_transcript` | After response | Reads transcript JSONL for passive capture, closes session |

On session start, mnemo detects the project from the git root directory name and emits relevant memories from previous sessions into the context.

---

## MCP tools

Tools available inside your editor via the `mcp__mnemo__*` namespace:

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
mnemo json <key> [key...]            Extract a field from JSON read from stdin (key path, array index supported)
mnemo extract-transcript <file>      Extract assistant text blocks from a JSONL transcript
mnemo setup [--dry-run]              Install hooks and configure Claude Code
mnemo setup --cursor [--dry-run]     Install hooks and configure Cursor
mnemo setup --windsurf [--dry-run]   Install hooks and configure Windsurf
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
