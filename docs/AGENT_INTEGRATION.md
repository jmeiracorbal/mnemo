# Agent integration

mnemo supports Claude Code, Cursor, Windsurf, Codex, OpenCode, and Pi through one controller-owned memory contract. MCP is used where the agent can carry its native session identity; Pi uses its native extension instead. Agent-specific runtime surfaces may provide context or durable events, but never direct SQLite writes.

## Global surfaces

| Surface | Claude Code | Cursor | Windsurf | Codex | OpenCode | Pi |
|---|---|---|---|---|---|---|
| **Hook scripts** | via plugin or n/a via `install.sh` | `~/.cursor/hooks/` | `~/.codeium/windsurf/hooks/` | `~/.codex/hooks/` | `~/.config/opencode/plugins/` | `~/.pi/agent/extensions/mnemo.ts` |
| **Hook config** | plugin hooks check `.mnemo` | `~/.cursor/hooks.json` | `~/.codeium/windsurf/hooks.json` | `~/.codex/hooks.json` checks `.mnemo` | global plugin checks `.mnemo` | `~/.pi/agent/extensions/mnemo.ts` |
| **Skill access** | symlinks under `~/.claude/skills/` | canonical `~/.agents/skills/` | symlinks under `~/.codeium/windsurf/skills/` | canonical `~/.agents/skills/` | canonical `~/.agents/skills/` | symlink under `~/.pi/agent/skills/` |

All supported agents use global hook/configuration surfaces where available. Their global instructions are conditional: if `.mnemo` is missing or invalid, agents skip mnemo entirely and do not create fallback memory files.

## Skill installation model

mnemo keeps one canonical global skill copy at `~/.agents/skills/mnemo-memory/`. Agent-specific skill directories never receive independent copies from mnemo; when an agent needs its own skill directory, the agent's `AgentSpec` declares a symlink to that canonical folder.

Current behavior:

- Claude Code, Windsurf and Pi receive symlinks from their agent-specific global skill directories to `~/.agents/skills/mnemo-memory/`.

MCP setup records lightweight provenance for supported configurations by setting `MNEMO_AGENT`, `MNEMO_MCP_CLIENT` and `MNEMO_MCP_TRANSPORT` in the generated MCP server entry. Pi is a native-extension exception: its extension supplies `agent=pi` and Pi's native session ID directly to the controller. mnemo stores that metadata in normalized SQLite tables separate from the project identity in `.mnemo`.

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
| `SessionStart` (startup/resume/clear) | New session | Injects memory context and deferred-tool loading protocol |
| `UserPromptSubmit` | Each user message | Re-emits ToolSearch on first message; periodic save reminders |
| `SessionStart` (compact) | After compaction | Recovers context from mnemo after context window reset |

### Cursor

| Hook | Trigger | Action |
|---|---|---|
| `beforeSubmitPrompt` | Prompt submission | Injects memory context and memory authority protocol |

### Windsurf

| Hook | Trigger | Action |
|---|---|---|
| `pre_user_prompt` | Prompt submission | Injects memory context and memory authority protocol |

### Codex

| Hook | Trigger | Action |
|---|---|---|
| `SessionStart` (startup/resume) | Session starts or resumes | Injects memory context via `systemMessage` |

### OpenCode

| Hook | Trigger | Action |
|---|---|---|
| `session.created` | New session created | Registers session with mnemo |
| `experimental.chat.system.transform` | First prompt of a conversation | Injects memory context into the system prompt |
| `experimental.session.compacting` | Context compaction | Refreshes context from mnemo, re-arms context injection |




### Pi

Pi support uses global `~/.pi/agent/APPEND_SYSTEM.md` instructions, project `AGENTS.md` activation, the canonical `mnemo-memory` skill linked into `~/.pi/agent/skills/`, and `~/.pi/agent/extensions/mnemo.ts`. The extension registers mnemo tools through Pi's supported `pi.registerTool` API and supplies `ctx.sessionManager.getSessionId()` to the controller on every call.

mnemo does not write `.pi/SYSTEM.md` because that file replaces Pi's default system prompt. The managed PI guidance is appended instead, so Pi keeps its default prompt, context files and skills behavior. Pi's extension is the lifecycle and tool surface; mnemo removes its legacy static `mcpServers.mnemo` entry because a generic MCP child cannot carry Pi's native session ID.

On session start, the extension reads the project identifier from `.mnemo` and publishes the native Pi session identity. The controller creates and closes the deterministic execution session; neither the extension nor a caller creates or passes a mnemo session ID.
