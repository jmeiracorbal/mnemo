# Windsurf memory surfaces

## Native model

Windsurf Cascade has two documented mechanisms for context across conversations:

1. Memories generated automatically by Cascade.
2. Rules manually defined by the user at global, workspace, AGENTS.md, or Enterprise system scope.

Windsurf's own recommendation is to use Rules or repository `AGENTS.md` for durable team knowledge instead of relying only on generated Memories.

## Locations

Generated Memories:

- Stored locally under `~/.codeium/windsurf/memories/`.
- Associated with the workspace where they were created.
- Not committed to the repository and not available in other workspaces.

Rules:

- Global rules: `~/.codeium/windsurf/memories/global_rules.md`.
- Workspace rules: `.windsurf/rules/**/*.md`.
- `AGENTS.md` in a workspace directory, processed by the same Rules engine.
- Enterprise system rules: OS-specific directories such as `/etc/windsurf/rules/*.md` on Linux/WSL or `/Library/Application Support/Windsurf/rules/*.md` on macOS.
- Legacy `.windsurfrules` may exist and should be detected best-effort.

## Shape and retrieval

- Generated Memories are local, workspace-associated items retrieved by Cascade when relevant.
- The public docs do not describe a `MEMORY.md`-style index for generated Memories; treat each discovered memory artifact as a candidate item and fail closed on unknown formats.
- Workspace rules are markdown files with frontmatter, including `trigger` modes such as `always_on`, `glob`, `model_decision`, or `manual`.
- Global rules and root-level `AGENTS.md` are always active and do not use frontmatter.
- Workspace rule files have documented size limits; global rules are shorter than workspace rules.

## Conversion notes for mnemo

Recommended import order:

1. `.windsurf/rules/**/*.md` and root/nested `AGENTS.md` files.
2. `~/.codeium/windsurf/memories/` generated Memories for the active workspace.
3. `global_rules.md` only with explicit user-scope opt-in.
4. Enterprise system rules should not be imported by default.

Suggested mapping:

- Generated Memories -> classify as `decision`, `discovery`, `pattern`, `preference`, or `manual` based on content.
- `always_on` rules -> `pattern` or `config` with activation provenance.
- `glob` rules -> `pattern` with source glob provenance.
- `model_decision` or `manual` rules -> `manual` unless the content is clearly a durable decision or convention.

Important importer behavior:

- Preserve workspace association.
- Preserve rule trigger, globs, and description as provenance.
- Treat generated Memories as local user data; do not import unrelated workspaces without explicit selection.

## Sources

- https://docs.windsurf.com/es/windsurf/cascade/memories
