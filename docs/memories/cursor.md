# Cursor memory surfaces

## Native model

Cursor's current documented durable context surface is Rules. Rules are prompt-level reusable context for Agent, not a structured memory database. Cursor documents four rule types: Project Rules, User Rules, Team Rules, and `AGENTS.md`.

Older/alternate Cursor memory references exist in the ecosystem, but the current public docs route durable project knowledge through Rules and `AGENTS.md`. A mnemo importer should therefore treat Cursor project rules as the primary conversion surface and avoid scraping opaque editor databases.

## Locations

Project rules:

- `.cursor/rules/**/*.mdc`.
- Each project rule must use the `.mdc` extension.
- A plain `.md` file inside `.cursor/rules` is ignored by Cursor's rule system unless it is referenced some other way.

Other durable surfaces:

- `AGENTS.md` as a simple markdown alternative to `.cursor/rules`.
- User Rules are global to the Cursor environment and managed from Cursor settings.
- Team Rules are dashboard-managed for Team/Enterprise plans.
- Legacy `.cursorrules` may exist in older projects and should be detected as a best-effort legacy import source.

## Shape and retrieval

- Project rules are markdown files with frontmatter.
- Frontmatter controls application through fields equivalent to `description`, `globs`, and `alwaysApply`.
- Rule behavior includes always apply, agent-selected by description, file-pattern attachment, or manual `@` mention.
- Rules are included at the start of model context when applied.
- Cursor rules can reference supporting files, for example with `@template-file` links. That is a rule-support pattern, not a central memory index.

## Conversion notes for mnemo

Recommended import order:

1. `.cursor/rules/**/*.mdc`.
2. Root and nested `AGENTS.md` files, if present.
3. Legacy `.cursorrules` if present.
4. User Rules / Team Rules only through a user-approved export or explicit path.

Suggested mapping:

- `alwaysApply: true` broad rules -> `pattern` or `config` after review.
- `globs` scoped rules -> `pattern` with a source scope derived from the glob.
- Rules with `description` but no glob -> `manual` or `pattern`, preserving description as provenance.
- Legacy `.cursorrules` -> split by headings/bullets and classify conservatively.

Important importer behavior:

- Parse frontmatter structurally.
- Preserve `alwaysApply`, `description`, and `globs` as SQL-queryable provenance.
- Preserve referenced `@file` targets separately from the rule body; follow them only in dry-run or with explicit import approval.
- Do not convert every rule into a factual observation without review; many rules are behavioral instructions.

## Sources

- https://prod.cursor.com/docs/rules
