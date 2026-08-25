# Agent integration

mnemo supports Claude Code, Cursor, Windsurf, Codex, OpenCode, fx and Pi through global setup surfaces plus project-local activation.

## Global surfaces

| | Claude Code | Cursor | Windsurf | Codex | OpenCode | fx | Pi |
|---|---|---|---|---|---|---|---|
| **Hook scripts** | via plugin or n/a via `install.sh` | `~/.cursor/hooks/` | `~/.codeium/windsurf/hooks/` | `~/.codex/hooks/` | `~/.config/opencode/plugins/` | n/a | n/a |
| **MCP** | `~/.claude/.mcp.json` | `~/.cursor/mcp.json` | `~/.codeium/windsurf/mcp_config.json` | `~/.codex/config.toml` | `~/.config/opencode/opencode.json` | `~/.fx/mcp.json` | `~/.pi/agent/mcp.json` via MCP extension |
| **Hook config** | plugin hooks check `.mnemo` | `~/.cursor/hooks.json` | `~/.codeium/windsurf/hooks.json` | `~/.codex/hooks.json` checks `.mnemo` | global plugin checks `.mnemo` | n/a | n/a |
| **Global protocol** | `~/.claude/CLAUDE.md` | `~/.cursor/rules/mnemo.mdc` | `~/.codeium/windsurf/memories/global_rules.md` | `~/.codex/AGENTS.md` | `~/.config/opencode/AGENTS.md` | `~/.fx/AGENTS.md` | `~/.pi/agent/APPEND_SYSTEM.md` |
| **Skill access** | symlinks under `~/.claude/skills/` | canonical `~/.agents/skills/` | symlinks under `~/.codeium/windsurf/skills/` | canonical `~/.agents/skills/` | canonical `~/.agents/skills/` | canonical `~/.agents/skills/` | symlink under `~/.pi/agent/skills/` |

All supported agents use global hook/configuration surfaces where available. Their global instructions are conditional: if `.mnemo` is missing or invalid, agents skip mnemo entirely and do not create fallback memory files.

## Skill installation model

mnemo keeps one canonical global skill copy at `~/.agents/skills/mnemo-memory/`. Agent-specific skill directories never receive independent copies from mnemo; when an agent needs its own skill directory, the agent's `AgentSpec` declares a symlink to that canonical folder.

Current behavior:

- Claude Code, Windsurf and Pi receive symlinks from their agent-specific global skill directories to `~/.agents/skills/mnemo-memory/`.
- Codex, Cursor, OpenCode and fx load `~/.agents/skills/` directly according to their current skill discovery docs, so mnemo does not add redundant agent-specific symlinks for them.

MCP setup records lightweight provenance for supported configurations by setting `MNEMO_AGENT`, `MNEMO_MCP_CLIENT` and `MNEMO_MCP_TRANSPORT` in the generated MCP server entry. mnemo stores that metadata in normalized SQLite tables separate from the project identity in `.mnemo`.

## What `mnemo init` creates

```text
project/
├── .mnemo                        ← project ID + configured agents
├── AGENTS.md                     ← mnemo memory authority (managed section)
├── CLAUDE.md                     ← Claude-specific block (when --agent=claudecode)
├── .cursor/rules/mnemo.mdc       ← Cursor project rules (when --agent=cursor)
├── .windsurf/rules/mnemo.md      ← Windsurf project rules (when --agent=windsurf)
└── .pi/APPEND_SYSTEM.md          ← Pi prompt extension (when --agent=pi)
```

By default, `mnemo init` writes project-level memory authority rules inside managed `<!-- mnemo:start -->` … `<!-- mnemo:end -->` sections, or dedicated Cursor/Windsurf/Pi prompt files. Use `--no-project-rules` to create only the `.mnemo` marker.

## The `.mnemo` marker

The `.mnemo` file at the project root activates mnemo for a project:

```json
{
  "version": 1,
  "id": "8ec0f7ec-7cf8-5f6c-a4dc-bb247f75c543",
  "agents": ["claudecode", "cursor"]
}
```

`id` is the deterministic project identifier used by every integration. `agents` lists which agents have been activated via `mnemo init`. All global hooks and plugins read this file before acting. If the file is absent or has no ID, the integration exits silently.

`mnemo init` creates and updates this file automatically and adds it to `.gitignore`. Do not commit it: each clone derives its own identifier from its local path.

## Hook behavior

### Claude Code

| Hook | Trigger | Action |
|---|---|---|
| `SessionStart` (startup/resume/clear) | New session | Registers session, injects memory context and deferred-tool loading protocol |
| `UserPromptSubmit` | Each user message | Re-emits ToolSearch on first message; periodic save reminders |
| `SessionStart` (compact) | After compaction | Recovers context from mnemo after context window reset |
| `PostCompact` | During compaction | Persists compaction summary to mnemo |
| `Stop` | Session ends | Marks session completed, warns if nothing was saved |
| `SubagentStop` | Subagent finishes | Passively captures learnings from subagent output |

### Cursor

| Hook | Trigger | Action |
|---|---|---|
| `beforeSubmitPrompt` | First prompt of a conversation | Registers session, injects memory context and memory authority protocol |
| `stop` | Conversation ends | Reads transcript JSONL for passive capture, closes session |

### Windsurf

| Hook | Trigger | Action |
|---|---|---|
| `pre_user_prompt` | First prompt of a conversation | Registers session, injects memory context and memory authority protocol |
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

### fx

fx support uses MCP, global `AGENTS.md` instructions and the canonical `mnemo-memory` skill under `~/.agents/skills/`. It does not install hooks because fx does not expose a supported hook surface for mnemo to rely on.

fx also has a native `memory` tool backed by `~/.fx/memories.json`. The mnemo-managed `~/.fx/AGENTS.md` block explicitly disables that native memory surface for repository/project memory whenever a valid `.mnemo` marker exists; agents must use mnemo MCP tools instead.

### Pi

Pi support uses global `~/.pi/agent/APPEND_SYSTEM.md` instructions, project `AGENTS.md` activation, the canonical `mnemo-memory` skill linked into `~/.pi/agent/skills/`, and a standard `mcpServers` entry in `~/.pi/agent/mcp.json` for environments that have a Pi MCP extension such as `pi-mcp-adapter`, `pi-mcp-extension`, or `pi-mcp` installed.

mnemo does not write `.pi/SYSTEM.md` because that file replaces Pi's default system prompt. The managed PI guidance is appended instead, so Pi keeps its default prompt, context files and skills behavior. Pi support does not install hooks because Pi does not expose a stable declarative hook surface for mnemo to rely on.

On session start, every hook resolves the Git root and reads the project identifier from `.mnemo`. This keeps the same identity regardless of which subdirectory the editor opens.
