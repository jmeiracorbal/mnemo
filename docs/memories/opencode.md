# OpenCode memory surfaces

## Native model

OpenCode V2 documents instructions as privileged context. It combines built-in context, discovered `AGENTS.md` files, and dynamic sources such as skills, references, MCP, and session context. For durable project guidance, OpenCode recommends `AGENTS.md`.

OpenCode does not document a separate user-facing semantic memory database. Its session instruction deltas are runtime/session infrastructure and should not be treated as the primary migration source.

## Locations

Global:

- `$XDG_CONFIG_HOME/opencode/AGENTS.md`, normally `~/.config/opencode/AGENTS.md`.

Project:

- Every `AGENTS.md` from the current Location up to and including the home directory when the Location is inside home.
- For Locations outside home, the scan stops at the project root.
- Nested `AGENTS.md` below the Location can be discovered later when a tool reads or lists a target path.

## Shape and retrieval

- Plain markdown `AGENTS.md` files.
- Files are combined rather than using a single winner.
- OpenCode renders global instructions first, then files from Location toward home or project root.
- It does not resolve conflicts between files; users should keep broad guidance global and scoped guidance near the relevant directory.
- Setting `OPENCODE_DISABLE_PROJECT_CONFIG=1` skips project `AGENTS.md` discovery but not the global file.
- There is no documented `MEMORY.md`-style index. Nested `AGENTS.md` discovery is target-scoped instruction loading, not a pointer to separate memory bodies.
- OpenCode V2 accepts an `instructions` array in config, but current docs state those entries are parsed/retained and not resolved into active instruction sources.

## Conversion notes for mnemo

Recommended import order:

1. Project `AGENTS.md` files from root/nested directories.
2. Global `~/.config/opencode/AGENTS.md` only with explicit user-scope opt-in.
3. Session internals should be ignored unless OpenCode later exposes a documented export.

Suggested mapping:

- Build/test commands -> `config`.
- Project structure and conventions -> `pattern`.
- Known gotchas and verification rules -> `discovery` or `config`.
- Instructions that only affect agent behavior -> `manual` unless they encode a durable project fact.

Important importer behavior:

- Preserve file scope and discovery direction.
- Do not assume conflict resolution; surface conflicts in dry-run if multiple AGENTS.md files contain contradictory guidance.
- Do not import session history/deltas as memories by default.

## Sources

- https://opencode.ai/v2/docs/instructions
