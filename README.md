# mnemo

[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Status](https://img.shields.io/badge/status-stable-brightgreen)](https://github.com/jmeiracorbal/mnemo)
[![Storage](https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white)](https://sqlite.org)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-blueviolet?logo=anthropic&logoColor=white)](https://claude.ai/code)
[![Cursor](https://img.shields.io/badge/Cursor-2.6%2B-000000?logo=cursor&logoColor=white)](https://cursor.com)
[![Windsurf](https://img.shields.io/badge/Windsurf-supported-0066CC)](https://codeium.com/windsurf)
[![Codex](https://img.shields.io/badge/Codex-supported-412991?logo=openai&logoColor=white)](https://openai.com/codex)
[![OpenCode](https://img.shields.io/badge/OpenCode-plugin-6D28D9)](https://opencode.ai)
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
- **Portable Agent Skill:** teaches compatible agents the complete mnemo workflow without weakening the always-active safety rules
- **Full CLI:** save, search, export, import, and inspect memories from the terminal
- **Own storage:** isolated `~/.mnemo/memory.db`, created automatically on first run
- **Claude Code + Cursor + Windsurf + Codex + OpenCode:** native integration for all five agents via their respective hook systems

## Setup

### 1. Install the binary and agent integration

```bash
# Claude Code (via plugin — recommended)
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo

# or via install.sh; default is --agent=auto, CodeGraph-style detection
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash

# explicit targets are still supported
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=codex
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=opencode
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=all
```

**Note:** The plugin integrations depend on the `mnemo` binary. Ensure that the installed binary is available in your system's `PATH`.

This installs hook scripts to the agent's global directory and registers the MCP server. Hooks are wired up once here, but they will do nothing in projects without a `.mnemo` marker.

### 2. Install the Agent Skill globally

For the best agent behavior, install the portable `mnemo-memory` skill:

```bash
npx skills add jmeiracorbal/mnemo --skill mnemo-memory --global
```

The [skills CLI](https://github.com/vercel-labs/skills) keeps the canonical copy at `~/.agents/skills/mnemo-memory/`. Codex and Cursor read that universal location directly; agent-specific consumers such as Claude Code and Windsurf receive symlinks to it.

The skill is global, but it is guarded by the project marker. Its first step is to locate and validate `.mnemo`; outside an initialized project it performs no memory operation and never falls back to native or plaintext memory.

The skill is recommended rather than required. Hooks, MCP, and the always-active project protocol remain functional without it. `mnemo init` prints the installation command when the canonical global skill is missing.

### 3. Enable mnemo per project

Run from the project root:

```bash
mnemo init --agent=claudecode   # or cursor, windsurf, codex, opencode, all
```

This creates a `.mnemo` marker and records the selected agent in it. Agent hooks, MCP configuration, and the short conditional protocol live globally after installation; they activate only when a project has a valid `.mnemo` file.

## Agent capabilities

mnemo follows a CodeGraph-style split: installation configures agents globally, while `mnemo init` only activates an individual project through `.mnemo`. The table below shows the global surfaces used by each agent.

| | Claude Code | Cursor | Windsurf | Codex | OpenCode |
|---|---|---|---|---|---|
| **Hook scripts (global)** | via plugin | `~/.cursor/hooks/` | `~/.codeium/windsurf/hooks/` | `~/.codex/hooks/` | `~/.config/opencode/plugins/` |
| **MCP (always global)** | `~/.claude/.mcp.json` | `~/.cursor/mcp.json` | `~/.codeium/windsurf/mcp_config.json` | `~/.codex/config.toml` | `~/.config/opencode/opencode.json` |
| **Hook config** | plugin hooks check `.mnemo` | `~/.cursor/hooks.json` | `~/.codeium/windsurf/hooks.json` | `~/.codex/hooks.json` checks `.mnemo` | global plugin checks `.mnemo` |
| **Global protocol** | `~/.claude/CLAUDE.md` | `~/.cursor/rules/mnemo.mdc` | `~/.codeium/windsurf/memories/global_rules.md` | `~/.codex/AGENTS.md` | `~/.config/opencode/AGENTS.md` |
| **Global skill access** | symlink at `~/.claude/skills/mnemo-memory` | canonical `~/.agents/skills/mnemo-memory` | symlink at `~/.codeium/windsurf/skills/mnemo-memory` | canonical `~/.agents/skills/mnemo-memory` | canonical `~/.agents/skills/mnemo-memory` |

All supported agents now use global hook/configuration surfaces where available. Their global instructions are intentionally conditional: if `.mnemo` is missing or invalid, agents skip mnemo entirely and do not create fallback memory files.

## What `mnemo init` creates

```
project/
└── .mnemo                        ← project ID + configured agents
```

`mnemo init` no longer appends mnemo protocol sections to project `AGENTS.md`, `CLAUDE.md`, Cursor rules, or Windsurf rules. Those instructions are installed globally by `install.sh` / `mnemo install-instructions` and are activated by the `.mnemo` marker.

## The `.mnemo` marker

The `.mnemo` file at the project root is what activates mnemo for a project. It is a small JSON file:

```json
{
  "version": 1,
  "id": "8ec0f7ec-7cf8-5f6c-a4dc-bb247f75c543",
  "agents": ["claudecode", "cursor"]
}
```

`id` is the deterministic project identifier used by every integration. `agents` lists which agents have been activated via `mnemo init`. All global hooks and plugins read this file before acting. If the file is absent or has no ID, the integration exits silently.

`mnemo init` creates and updates this file automatically and adds it to `.gitignore`. Do not commit it: each clone derives its own identifier from its local path.

## Claude Code plugin notes

**Via plugin (recommended):**

```bash
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo
```

Requires the `mnemo` binary in PATH. Restart Claude Code after installing. Then run `mnemo init --agent=claudecode` from each project.

**Updating the plugin:**

```bash
claude plugin update mnemo@mnemo
```

If that reports up to date but the binary is newer, the marketplace cache is stale. Reinstall:

```bash
claude plugin uninstall mnemo && claude plugin install mnemo@mnemo
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

### OpenCode

| Hook | Trigger | Action |
|---|---|---|
| `session.created` | New session created | Registers session with mnemo |
| `experimental.chat.system.transform` | First prompt of a conversation | Injects memory context into the system prompt |
| `experimental.session.compacting` | Context compaction | Refreshes context from mnemo, re-arms context injection |

On session start, every hook resolves the Git root and reads the project identifier from its `.mnemo` marker. This keeps the same identity regardless of which subdirectory the editor opens.

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
mnemo init [--agent=AGENT]           Activate mnemo in the current project (.mnemo)
mnemo install-instructions [--agent=AGENT]  Install global agent instructions
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

Agents for `mnemo init` / `mnemo install-instructions`:

```
--agent=claudecode   Claude Code global CLAUDE.md instructions / .mnemo activation (default for init)
--agent=cursor       Cursor global rule / .mnemo activation
--agent=windsurf     Windsurf global rule / .mnemo activation
--agent=codex        Codex global AGENTS.md instructions / .mnemo activation
--agent=opencode     OpenCode global AGENTS.md instructions / .mnemo activation
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

Confirm the canonical Agent Skill and agent symlinks:

```bash
test -f ~/.agents/skills/mnemo-memory/SKILL.md
ls -l ~/.claude/skills/mnemo-memory \
  ~/.codeium/windsurf/skills/mnemo-memory
```

Only symlinks for agent-specific consumers selected during `npx skills add` are expected to exist. Codex and Cursor use the canonical `.agents/skills` path directly.

After running the installer, check global agent files exist; after `mnemo init`, check only the project marker:

```bash
# Project activation
cat .mnemo                          # must contain id + agents list

# Global instructions
grep "mnemo:start" ~/.codex/AGENTS.md ~/.claude/CLAUDE.md 2>/dev/null
head -3 ~/.cursor/rules/mnemo.mdc   # should have: alwaysApply: true
grep "mnemo:start" ~/.codeium/windsurf/memories/global_rules.md 2>/dev/null
grep "mnemo:start" ~/.config/opencode/AGENTS.md 2>/dev/null

# Global hooks/config
grep "mnemo" ~/.cursor/hooks.json ~/.codeium/windsurf/hooks.json ~/.codex/hooks.json 2>/dev/null
ls ~/.config/opencode/plugins/mnemo.ts ~/.config/opencode/plugins/mnemo-protocol.md
```

Validate the Claude Code plugin:

```bash
claude plugin validate plugin/claude-code
```

Check idempotency. Running `mnemo install-instructions` or `mnemo init` again must not duplicate managed sections or marker entries:

```bash
mnemo install-instructions --agent=codex
mnemo install-instructions --agent=codex  # second run: no duplicate block
mnemo init --agent=claudecode
mnemo init --agent=claudecode             # second run: no duplicate agent entry
```

## Storage

mnemo uses `~/.mnemo/memory.db`, created automatically on first run. The schema uses SQLite with FTS5 for full-text search.
Runtime queries are defined under `internal/db/queries` and compiled into type-safe Go code with sqlc.
After changing the schema or a query, regenerate and test with:

```bash
go tool sqlc generate
git diff --exit-code -- internal/db/generated
go test ./...
```

## License

[Apache 2.0](LICENSE): you may use, modify, and distribute freely, but must retain the copyright notice and include the [NOTICE](NOTICE) file in all distributions.
