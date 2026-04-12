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

## Prerequisite: binary in PATH

The `mnemo` binary must be in your `PATH` before any agent integration will work. This applies to Claude Code, Cursor, and Windsurf regardless of how the integration is installed.

The hooks that fire on session start, session end, and passive capture all call `mnemo` directly. The MCP server is also the `mnemo` binary. Without it in PATH, hooks fail silently and the MCP server cannot start.

Install the binary:

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

## Features

- **Session hooks:** starts/ends sessions and injects memory context at the beginning of every conversation
- **18 MCP tools:** `mem_save`, `mem_search`, `mem_context`, `mem_tag_stats`, and more, available directly inside your editor
- **Passive capture:** extracts learnings from conversation transcripts automatically at session end
- **Full CLI:** save, search, export, import, and inspect memories from the terminal
- **Own storage:** isolated `~/.mnemo/memory.db`, created automatically on first run
- **Claude Code + Cursor + Windsurf:** native integration for all three editors via their respective hook systems

## Agent setup

### Claude Code

Two installation paths:

**Via plugin (recommended):**

Requires the `mnemo` binary in PATH — install it first following the [prerequisite step](#prerequisite-binary-in-path).

```bash
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo
```

The plugin registers the MCP server, hooks (SessionStart, Stop, SubagentStop, PostCompact), and writes the memory protocol to `~/.claude/mnemo.md` and `~/.claude/CLAUDE.md` on first session start.

**Updating the plugin:**

The marketplace cache can go stale and report an outdated version as the latest. If `claude plugin update mnemo@mnemo` says the plugin is already up to date but the binary is newer, refresh the marketplace first:

```bash
claude plugin marketplace update mnemo
claude plugin update mnemo@mnemo
```

Restart Claude Code after updating.

**Via setup command:**

```bash
mnemo setup
```

This writes hook scripts to `~/.claude/hooks/`, registers the MCP server via `claude mcp add -s user mnemo`, injects hooks into `~/.claude/settings.json`, adds all `mcp__mnemo__*` tools to `permissions.allow`, writes `~/.claude/mnemo.md`, and appends `@mnemo.md` to `~/.claude/CLAUDE.md`.

```bash
mnemo setup --dry-run   # preview without changes
```

### Cursor

```bash
mnemo setup --cursor
```

This writes hook scripts to `~/.cursor/hooks/`, registers the MCP server in `~/.cursor/mcp.json`, adds hooks to `~/.cursor/hooks.json` (beforeSubmitPrompt, stop), and writes `~/.cursor/rules/mnemo.mdc` (memory protocol, `alwaysApply: true`).

```bash
mnemo setup --cursor --dry-run
```

Restart Cursor after setup.

### Windsurf

```bash
mnemo setup --windsurf
```

This writes hook scripts to `~/.codeium/windsurf/hooks/`, registers the MCP server in `~/.codeium/windsurf/mcp_config.json`, adds hooks to `~/.codeium/windsurf/hooks.json` (pre_user_prompt, post_cascade_response_with_transcript), and appends the memory protocol to `~/.codeium/windsurf/memories/global_rules.md`.

```bash
mnemo setup --windsurf --dry-run
```

Restart Windsurf after setup.

## Verification checklist

Run this after every installation or update.

Check that the binary is accessible:

```bash
mnemo --version
```

Verify dry-runs produce no errors:

```bash
mnemo setup --dry-run
mnemo setup --cursor --dry-run
mnemo setup --windsurf --dry-run
```

Validate the plugin (Claude Code):

```bash
claude plugin validate plugin/claude-code
```

Check that protocol files exist with the right content:

```bash
cat ~/.claude/CLAUDE.md                          # must contain: @mnemo.md
head -1 ~/.claude/mnemo.md                      # must be: ## mnemo — Persistent Memory Protocol
head -3 ~/.cursor/rules/mnemo.mdc               # must have: alwaysApply: true
grep "mnemo — Persistent Memory Protocol" \
  ~/.codeium/windsurf/memories/global_rules.md  # must match
```

Check idempotency — running setup again must report "already up to date":

```bash
mnemo setup --cursor
mnemo setup --windsurf
```

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

## MCP tools

Tools available inside your editor via the `mcp__mnemo__*` namespace:

### Agent profile (default, 15 tools)

| Tool | Description |
|---|---|
| `mem_save` | Save a memory with title, content, type, tags, and optional topic key |
| `mem_search` | Search memories by text, tags, topic key, or any combination |
| `mem_context` | Retrieve formatted context from previous sessions |
| `mem_session_summary` | Save an end-of-session summary with goal, discoveries, next steps |
| `mem_session_start` | Register a new session |
| `mem_session_end` | Mark a session as completed, optionally with tags |
| `mem_get_observation` | Retrieve full content of a memory by ID |
| `mem_suggest_topic_key` | Suggest a topic key for deduplication |
| `mem_capture_passive` | Extract and save learnings from free-form text |
| `mem_save_prompt` | Save a prompt template |
| `mem_update` | Update an existing memory, including tags |
| `mem_list_tags` | List all tags in use for a project, ordered by frequency |
| `mem_merge_tags` | Merge all occurrences of one tag into another |
| `mem_tag_stats` | Query tag observability: top tags, low-frequency tags, stale tags |
| `mem_related_tags` | Find tags that co-occur with a given tag across observations and sessions |

### Admin profile (3 tools)

Available with `--tools=admin`: `mem_delete`, `mem_stats`, `mem_timeline`.

### Search modes

`mem_search` supports several independent intents that compose:

| Parameter | Type | Semantics |
|---|---|---|
| `query` | text, optional | Full-text search via FTS5. Omit to browse by other filters. |
| `tags` | comma list | Hard filter. Only observations that have **all** listed tags are returned. |
| `prefer_tags` | comma list | Soft signal. Observations matching more of these tags rank higher. Non-matching results are still returned. |
| `topic_key` | string | Hard filter. Only observations with this exact topic key. |
| `type` | string | Hard filter by observation type (e.g. `decision`, `bugfix`). |
| `project` | string | Scope to a project. |

`mem_context` accepts `tags`, `prefer_tags`, and `topic_key` with the same semantics, applied to recent observation retrieval.

To add the admin profile as a separate MCP server:

```bash
claude mcp add -s user mnemo-admin -- ~/.local/bin/mnemo mcp --tools=admin
```

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

Save a decision manually:

```bash
mnemo save "Use FTS5 for search" "Chose SQLite FTS5 over external search" --type decision --project myapp
```

Search memories:

```bash
mnemo search "authentication" --project myapp --limit 5
```

Show context from previous sessions:

```bash
mnemo context myapp
```

Export everything to JSON:

```bash
mnemo export backup.json
```

## Storage

mnemo uses `~/.mnemo/memory.db`, created automatically on first run. The schema uses SQLite with FTS5 for full-text search.

## License

[Apache 2.0](LICENSE): you may use, modify, and distribute freely, but must retain the copyright notice and include the [NOTICE](NOTICE) file in all distributions.
