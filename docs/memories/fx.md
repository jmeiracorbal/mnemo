# fx memory surfaces

## Native model

fx has two relevant durable surfaces:

1. Project instructions loaded from `AGENTS.md` files.
2. A built-in `memory` tool that stores durable user preferences in `~/.fx/memories.json`.

The fx docs describe `AGENTS.md` as project guidance. The fx source shows the memory tool supports `save`, `list`, and `clear`; it stores a JSON array of strings and resolves the path through the profile path helper.

## Locations

Instructions:

- Global instructions: `~/.fx/AGENTS.md`.
- Launch-ancestor and primary-workspace `AGENTS.md` files.
- More specific `AGENTS.md` files for files or directories targeted by a tool call.

Native memory tool:

- `~/.fx/memories.json`.

Other state:

- fx has sessions, history, logs, usage, MCP config, and credentials under `~/.fx/`, but those are not memory-import surfaces for project knowledge unless a future fx export documents them.

## Shape and retrieval

Instructions:

- Plain markdown `AGENTS.md` files.
- The narrowest applicable project scope wins when instructions conflict.
- fx assembles bounded context from recent/compacted conversation history, applicable `AGENTS.md`, skills, MCP metadata, tools, and the selected model.

Native memory tool:

- JSON array of strings.
- `save` requires a `fact` string and deduplicates by exact byte equality.
- `list` returns all entries as bullet lines.
- `clear` removes the file and is irreversible.
- The tool description says it is for durable user preferences, not task notes, secrets, project facts, or temporary context.
- There is no index layer: `memories.json` itself is the store and each array element is a complete memory string.

## Conversion notes for mnemo

Recommended import order:

1. Project `AGENTS.md` files.
2. `~/.fx/memories.json` only with explicit user approval because it is intended for user preferences.
3. `~/.fx/AGENTS.md` only with explicit user-scope opt-in.

Suggested mapping:

- Entries from `~/.fx/memories.json` -> `preference` by default.
- Project `AGENTS.md` facts -> `pattern`, `config`, or `decision` after classification.
- Avoid importing `~/.fx/memories.json` entries as project memory unless the user confirms they apply to the current project.

Important importer behavior:

- Parse `memories.json` strictly as a JSON array of strings.
- Preserve array index as source ordering because the entries have no IDs or timestamps.
- Detect malformed, oversized, or non-array files and fail loudly in dry-run rather than treating them as empty.
- Preserve whether the memory came from fx's user preference tool versus an instruction file.

## Sources

- https://fx.sh/docs/configure-fx/project-instructions
- https://raw.githubusercontent.com/vercel-labs/fx/main/src/tools/memory/memory.zig
- https://raw.githubusercontent.com/vercel-labs/fx/main/src/core/shared/profile_paths.zig
- https://raw.githubusercontent.com/vercel-labs/fx/main/src/builtins/tools.zig
