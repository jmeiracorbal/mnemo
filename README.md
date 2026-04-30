# mnemo

[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Status](https://img.shields.io/badge/status-stable-brightgreen)](https://github.com/jmeiracorbal/mnemo)
[![Storage](https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white)](https://sqlite.org)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-blueviolet?logo=anthropic&logoColor=white)](https://claude.ai/code)
[![Cursor](https://img.shields.io/badge/Cursor-2.6%2B-000000?logo=cursor&logoColor=white)](https://cursor.com)
[![Windsurf](https://img.shields.io/badge/Windsurf-supported-0066CC)](https://codeium.com/windsurf)
[![Codex](https://img.shields.io/badge/Codex-supported-412991?logo=openai&logoColor=white)](https://openai.com/codex)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/jmeiracorbal/mnemo)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Persistent memory for AI coding agents. mnemo stores decisions, bugs, conventions, and discoveries across sessions in a local SQLite database.

The integration is **per-project and opt-in**. Hooks fire globally on every session, but they only activate when the project contains a `.mnemo` marker. Run `mnemo init` once from a project root to enable mnemo there. Projects without the marker are unaffected.

## Prerequisite: binary in PATH

The `mnemo` binary must be in your `PATH` before any agent integration will work. This applies to Claude Code, Cursor, Windsurf, and Codex regardless of how the integration is installed.

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
- **Claude Code + Cursor + Windsurf + Codex:** native integration for all four agents via their respective hook systems

## Setup: two phases

**Phase 1 — install globally (once per machine):**

```bash
# Claude Code (via plugin — recommended)
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo

# or via install.sh for any agent:
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=claudecode
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=cursor
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=windsurf
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=codex
```

This installs hook scripts to the agent's global directory and registers the MCP server. Hooks are wired up once here, but they will do nothing in projects without a `.mnemo` marker.

**Phase 2 — enable per project (once per project):**

Run from the project root:

```bash
mnemo init --agent=claudecode   # or cursor, windsurf, codex, all
```

This creates a `.mnemo` marker, writes the agent protocol file, and configures per-project hook settings where the agent supports it. From this point on, hooks will fire when working in this project.

## Agent capabilities

Not all agents support per-project hook configuration. The table below shows what each phase can configure per agent.

| | Claude Code | Cursor | Windsurf | Codex |
|---|---|---|---|---|
| **Hook scripts (global)** | via plugin | `~/.cursor/hooks/` | `~/.codeium/windsurf/hooks/` | `~/.codex/hooks/` |
| **MCP (always global)** | `~/.claude/.mcp.json` | `~/.cursor/mcp.json` | `~/.codeium/windsurf/mcp_config.json` | `~/.codex/config.toml` |
| **Per-project hook config** | `.claude/settings.json` | `.cursor/hooks.json` | `.windsurf/hooks.json` | ❌ global only |
| **Per-project protocol** | `AGENTS.md` + `CLAUDE.md` symlink | `.cursor/rules/mnemo.mdc` | `.windsurf/rules/mnemo.md` | `AGENTS.md` |

Codex does not support per-project hook configuration. Its hooks are registered globally in `~/.codex/hooks.json` and check for `.mnemo` at runtime before acting.

## What `mnemo init` creates per agent

### Claude Code

```
project/
├── .mnemo                   ← {"version":1,"agents":["claudecode"]}
├── AGENTS.md                ← mnemo protocol section appended here
├── CLAUDE.md -> AGENTS.md   ← symlink (existing content migrated to AGENTS.md)
└── .claude/
    └── settings.json        ← hook entries (SessionStart, Stop, PostCompact, SubagentStop)
```

`CLAUDE.md` becomes a symlink to `AGENTS.md`. If `CLAUDE.md` already exists as a regular file, its content is moved to `AGENTS.md` before the symlink is created. This keeps all project config in a single file that works for both Claude Code and Codex.

### Cursor

```
project/
├── .mnemo                        ← marker (agents includes "cursor")
└── .cursor/
    ├── hooks.json                ← beforeSubmitPrompt + stop hooks
    └── rules/
        └── mnemo.mdc             ← protocol as a Cursor rule (alwaysApply: true)
```

`hooks.json` references the global scripts installed to `~/.cursor/hooks/` with their absolute paths.

### Windsurf

```
project/
├── .mnemo                        ← marker (agents includes "windsurf")
└── .windsurf/
    ├── hooks.json                ← pre_user_prompt + post_cascade_response_with_transcript hooks
    └── rules/
        └── mnemo.md              ← protocol as a Windsurf workspace rule
```

`hooks.json` references the global scripts installed to `~/.codeium/windsurf/hooks/`.

### Codex

```
project/
├── .mnemo                        ← marker (agents includes "codex")
└── AGENTS.md                     ← mnemo protocol section appended here
```

Hooks remain in `~/.codex/hooks.json`. The session-start and stop scripts check for `.mnemo` at runtime and skip if the marker is absent.

## The `.mnemo` marker

The `.mnemo` file at the project root is what activates mnemo for a project. It is a small JSON file:

```json
{
  "version": 1,
  "agents": ["claudecode", "cursor"]
}
```

`agents` lists which agents have been configured via `mnemo init`. All hooks — including the global-only Codex hooks — read this file before acting. If the file is absent, the hook exits silently.

`mnemo init` creates and updates this file automatically. You can commit it to the repository if you want the rest of your team to know mnemo is in use.

## Claude Code plugin notes

**Via plugin (recommended):**

```bash
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo
```

Requires the `mnemo` binary in PATH. Restart Claude Code after installing. Then run `mnemo init --agent=claudecode` from each project.

**Updating the plugin:**

The marketplace cache can go stale. If `claude plugin update mnemo@mnemo` says the plugin is already up to date but the binary is newer, refresh first:

```bash
claude plugin marketplace update mnemo
claude plugin update mnemo@mnemo
```

Restart Claude Code after updating.

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

### Codex

| Hook | Trigger | Action |
|---|---|---|
| `SessionStart` (startup/resume) | Session starts or resumes | Registers session, injects memory context via `systemMessage` |
| `Stop` | Agent stops | Reads transcript for passive capture, closes session |

On session start, mnemo derives the project identifier from the git root and emits relevant memories from previous sessions into the context. All hooks derive the project the same way — from the git root, not the current working directory — so the project ID is consistent regardless of which subdirectory the editor opens.

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
mnemo init [--agent=AGENT]           Configure mnemo in the current project
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
mnemo version                        Show version
```

Agents for `mnemo init`:

```
--agent=claudecode   AGENTS.md + CLAUDE.md symlink + .claude/settings.json (default)
--agent=cursor       .cursor/hooks.json + .cursor/rules/mnemo.mdc
--agent=windsurf     .windsurf/hooks.json + .windsurf/rules/mnemo.md
--agent=codex        AGENTS.md append only (hooks stay global)
--agent=all          All agents
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

## Verification

After installing globally, confirm the binary is accessible:

```bash
mnemo --version
```

After running `mnemo init`, check the project files exist:

```bash
# Claude Code
cat .mnemo                          # must contain agents list
ls -la CLAUDE.md                    # must be a symlink to AGENTS.md
grep "mnemo" .claude/settings.json  # must have hook entries

# Cursor
cat .cursor/hooks.json              # must have beforeSubmitPrompt + stop
head -3 .cursor/rules/mnemo.mdc    # must have: alwaysApply: true

# Windsurf
cat .windsurf/hooks.json            # must have pre_user_prompt + post_cascade_response_with_transcript
ls .windsurf/rules/mnemo.md

# Codex
grep "mnemo" AGENTS.md             # must have the protocol section
```

Validate the Claude Code plugin:

```bash
claude plugin validate plugin/claude-code
```

Check idempotency — running `mnemo init` again in the same project must produce no duplicates in `AGENTS.md` and must not overwrite the symlink:

```bash
mnemo init --agent=claudecode
mnemo init --agent=claudecode  # second run: no changes
```

## Storage

mnemo uses `~/.mnemo/memory.db`, created automatically on first run. The schema uses SQLite with FTS5 for full-text search.

## License

[Apache 2.0](LICENSE): you may use, modify, and distribute freely, but must retain the copyright notice and include the [NOTICE](NOTICE) file in all distributions.
