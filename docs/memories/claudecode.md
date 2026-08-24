# Claude Code memory surfaces

## Native model

Claude Code has two durable context systems:

1. `CLAUDE.md` files and `.claude/rules/` files for user- or project-authored instructions.
2. Auto memory files that Claude writes itself when it decides a fact is worth preserving.

The official docs describe `CLAUDE.md` files as persistent context and auto memory as notes Claude writes based on corrections and preferences. Auto memory is enabled by default and can be disabled via `/memory`, settings, or `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1`.

## Locations

Instruction files:

- Managed policy: `/Library/Application Support/ClaudeCode/CLAUDE.md`, `/etc/claude-code/CLAUDE.md`, or `C:\Program Files\ClaudeCode\CLAUDE.md`.
- User instructions: `~/.claude/CLAUDE.md`.
- User rules: `~/.claude/rules/**/*.md`.
- Project instructions: `./CLAUDE.md` or `./.claude/CLAUDE.md`.
- Project rules: `./.claude/rules/**/*.md`.
- Local project preferences: `./CLAUDE.local.md`.

Auto memory:

- Default: `~/.claude/projects/<project>/memory/`.
- The directory contains `MEMORY.md` as an index/entrypoint and one markdown topic file per memory.
- `MEMORY.md` is not necessarily the full memory body. It is the startup-loaded index that points Claude to the more detailed topic files that hold the real memory references/content.
- The project key is derived from the git repository; worktrees/subdirectories of the same repository share the same auto memory directory.
- `autoMemoryDirectory` can override the storage directory.

## Shape and retrieval

- `CLAUDE.md` and rules are markdown instruction files.
- `CLAUDE.md` can import additional files with `@path` syntax.
- `.claude/rules/*.md` files may have YAML frontmatter, including path scoping through `paths`.
- Auto memory files are markdown. Files with YAML frontmatter can include a `type` field such as `user`, `feedback`, `project`, or `reference`, and Claude Code can update `modified` timestamps.
- The first 200 lines or first 25 KiB of auto-memory `MEMORY.md` are loaded at the start of a conversation; topic files are read on demand.
- For ingestion, treat `MEMORY.md` as a manifest/index first: parse its entries and follow same-directory markdown references before deciding that an entry has no detail file.
- Topic files are the safer unit for full-fidelity import because they can contain the detailed memory body, frontmatter type, and modified timestamp.

## Conversion notes for mnemo

Recommended import order:

1. Project `CLAUDE.md` / `.claude/CLAUDE.md` and project rules.
2. `CLAUDE.local.md` only when the user explicitly opts in.
3. Auto memory `MEMORY.md` index, then the topic files it references.
4. User-level files only with explicit `--scope=user` or equivalent.

Suggested mapping:

- Auto memory frontmatter `type: user` -> `preference` unless content indicates config.
- `type: feedback` -> `pattern` or `preference` depending on wording.
- `type: project` -> `decision`, `discovery`, `config`, or `manual` after classification.
- `type: reference` -> `discovery` with source URL/path provenance.
- Instruction files -> usually `pattern` or `config`; avoid importing broad policy text as factual memory without review.

Important importer behavior:

- Preserve `source_file`, heading path, frontmatter fields, and import timestamp.
- When importing auto memory, preserve both the index source (`MEMORY.md`) and the referenced topic file path so users can audit where the observation came from.
- Expand `@path` imports only in dry-run unless explicitly allowed; imported files may point outside the project.
- Do not import managed policy files by default.

## Sources

- https://code.claude.com/docs/en/memory
