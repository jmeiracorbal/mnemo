# Agent integration

mnemo supports Claude Code, Cursor, Windsurf, Codex and OpenCode through global setup surfaces plus project-local activation.

## Global surfaces

| | Claude Code | Cursor | Windsurf | Codex | OpenCode |
|---|---|---|---|---|---|
| **Hook scripts** | via plugin or n/a via `install.sh` | `~/.cursor/hooks/` | `~/.codeium/windsurf/hooks/` | `~/.codex/hooks/` | `~/.config/opencode/plugins/` |
| **MCP** | `~/.claude/.mcp.json` | `~/.cursor/mcp.json` | `~/.codeium/windsurf/mcp_config.json` | `~/.codex/config.toml` | `~/.config/opencode/opencode.json` |
| **Hook config** | plugin hooks check `.mnemo` | `~/.cursor/hooks.json` | `~/.codeium/windsurf/hooks.json` | `~/.codex/hooks.json` checks `.mnemo` | global plugin checks `.mnemo` |
| **Global protocol** | `~/.claude/CLAUDE.md` | `~/.cursor/rules/mnemo.mdc` | `~/.codeium/windsurf/memories/global_rules.md` | `~/.codex/AGENTS.md` | `~/.config/opencode/AGENTS.md` |
| **Skill access** | symlinks under `~/.claude/skills/` | canonical `~/.agents/skills/` | symlinks under `~/.codeium/windsurf/skills/` | canonical `~/.agents/skills/` | canonical `~/.agents/skills/` |

All supported agents use global hook/configuration surfaces where available. Their global instructions are conditional: if `.mnemo` is missing or invalid, agents skip mnemo entirely and do not create fallback memory files.

## What `mnemo init` creates

```text
project/
├── .mnemo                        ← project ID + configured agents
├── AGENTS.md                     ← mnemo memory authority (managed section)
├── CLAUDE.md                     ← Claude-specific block (when --agent=claudecode)
├── .cursor/rules/mnemo.mdc       ← Cursor project rules (when --agent=cursor)
└── .windsurf/rules/mnemo.md      ← Windsurf project rules (when --agent=windsurf)
```

By default, `mnemo init` writes project-level memory authority rules inside managed `<!-- mnemo:start -->` … `<!-- mnemo:end -->` sections, or dedicated Cursor/Windsurf rule files. Use `--no-project-rules` to create only the `.mnemo` marker.

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

On session start, every hook resolves the Git root and reads the project identifier from `.mnemo`. This keeps the same identity regardless of which subdirectory the editor opens.
