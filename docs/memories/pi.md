# Pi memory surfaces

## Native model

Pi exposes durable context mostly through instruction and customization files rather than a documented semantic memory database.

Relevant surfaces:

1. Context files loaded at startup, especially `AGENTS.md` and `CLAUDE.md`.
2. System prompt files: `.pi/SYSTEM.md` replaces the default system prompt, while `.pi/APPEND_SYSTEM.md` extends it.
3. Skills under `~/.pi/agent/skills/`, `~/.agents/skills/`, `.pi/skills/`, and project `.agents/skills/`.
4. Session JSONL under `~/.pi/agent/sessions/`, which is resumable conversation history rather than curated semantic memory.
5. Optional MCP support through community extensions such as `pi-mcp-adapter`, `pi-mcp-extension`, or `pi-mcp`, which read standard MCP configuration.

mnemo treats Pi as an agent runtime with instruction, skill, and optional MCP surfaces.

## Locations

Instructions:

- Global context: `~/.pi/agent/AGENTS.md`.
- Project context: parent/current `AGENTS.md` or `CLAUDE.md` files.
- Project system replacement: `.pi/SYSTEM.md`.
- Project system extension: `.pi/APPEND_SYSTEM.md`.
- Global system replacement/extension: `~/.pi/agent/SYSTEM.md`, `~/.pi/agent/APPEND_SYSTEM.md`.

Skills:

- Global skills: `~/.pi/agent/skills/**/SKILL.md`.
- Project skills: `.pi/skills/**/SKILL.md`.
- Shared global skills: `~/.agents/skills/**/SKILL.md`.
- Project shared skills: `.agents/skills/**/SKILL.md` from the current working directory up through parent directories.
- Pi also documents package-provided, settings-provided, and CLI-provided skills.

Sessions:

- Session history: `~/.pi/agent/sessions/`, organized by working directory.
- Pi can export/import sessions as JSONL, but these are transcripts and branches, not a curated memory database.

MCP:

- Pi does not rely on a documented built-in MCP surface in the same way as Codex, Cursor, or Claude Code.
- Pi MCP adapters read standard MCP config from paths such as `.pi/mcp.json`, `.pi/mcp.jsonc`, and `~/.pi/agent/mcp.json`; mnemo writes the global Pi entry to `~/.pi/agent/mcp.json` using `mcpServers.mnemo`.

## Shape and retrieval

Instructions:

- Plain markdown files.
- `AGENTS.md` / `CLAUDE.md` files are concatenated from global and project hierarchy.
- `SYSTEM.md` is a full system-prompt replacement and can suppress default prompt behavior if used incorrectly.
- `APPEND_SYSTEM.md` is the safer extension point for mnemo-managed global guidance.

Skills:

- `SKILL.md` files use the Agent Skills-style markdown package shape.
- Skill availability depends on Pi's skill discovery, project trust, and active tool set.
- Context files load before project trust, while project-local `.pi` resources and project `.agents/skills` depend on the Pi trust flow.
- Skills are capability packages, not observations; import their metadata/instructions only when converting agent behavior, not as factual project memory by default.

Sessions:

- JSONL sessions preserve conversation history, branching, compaction, and tool calls.
- Treat sessions as audit/export material. Do not ingest them automatically as memory because they can contain stale plans, failed attempts, secrets, or transient user prompts.

Native memory:

- No documented local semantic memory store equivalent to Claude Code auto memory, Windsurf Memories, or fx `memories.json` is treated as an import target today.
- Pi context files may contain long-lived project knowledge, but they are instruction surfaces and should be classified before importing into mnemo observations.

## Conversion notes for mnemo

Recommended import order:

1. Project `AGENTS.md` / `CLAUDE.md` context files.
2. `.pi/APPEND_SYSTEM.md` because it extends the default prompt and is likely policy/guidance.
3. `.pi/SYSTEM.md` only with explicit review because it replaces the entire system prompt.
4. Global `~/.pi/agent/*` instruction files only with explicit user-scope opt-in.
5. Session JSONL only through an explicit transcript-import mode, never through the default memory importer.

Suggested mapping:

- Context-file behavioral rules -> `pattern` or `config` after classification.
- Project decisions embedded in context files -> `decision` only after a dry-run preview.
- User preferences in global files -> `preference` only with user-scope confirmation.

Important importer behavior:

- Preserve whether a source was `AGENTS.md`, `CLAUDE.md`, `SYSTEM.md`, or `APPEND_SYSTEM.md` because the runtime semantics differ.
- Prefer `APPEND_SYSTEM.md` for generated mnemo integration guidance; avoid writing `SYSTEM.md` so mnemo does not replace Pi's default prompt.
- Treat Pi MCP as conditional on an installed MCP extension until Pi documents a built-in MCP config contract.
- Preserve source path, heading, scope, and whether the file was global or project-local.
- Preserve project-trust context for `.pi/skills`, `.pi/settings.json`, and project `.agents/skills` if those surfaces are ever imported.
- Do not treat Pi sessions, package manifests, model settings, keybindings, themes, or extension code as memory import sources by default.

## Sources

- https://github.com/earendil-works/pi
- https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md
- https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/skills.md
- https://vercel.com/blog/ai-sdk-7
- https://www.npmjs.com/package/pi-mcp-adapter
- https://www.npmjs.com/package/pi-mcp-extension
- https://github.com/dmmulroy/pi-mcp
