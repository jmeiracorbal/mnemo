# Codex memory surfaces

## Native model

Codex does not expose a separate local semantic memory store comparable to mnemo. Its durable, user-controlled memory-like surface is instruction aggregation from `AGENTS.md` / `AGENTS.override.md` files plus configured fallback filenames.

OpenAI's agent-loop documentation describes Codex adding user instructions from `$CODEX_HOME` and then from folders between the Git/project root and the current working directory, subject to a default 32 KiB project-doc budget. The Codex source describes the same discovery path and concatenation model.

## Locations

Global/user scope:

- `$CODEX_HOME/AGENTS.override.md`, otherwise `$CODEX_HOME/AGENTS.md`.
- `$CODEX_HOME` is normally `~/.codex`.

Project scope:

- For each directory from project root to current working directory: `AGENTS.override.md`, then `AGENTS.md`, then configured `project_doc_fallback_filenames`.
- The default project root marker is `.git` when no custom `project_root_markers` are configured.

## Shape and retrieval

- Plain markdown instruction files.
- Files are concatenated from broad to narrow scope, so more specific files appear later.
- Empty files are skipped.
- Project docs are bounded by `project_doc_max_bytes`.
- Codex does not walk past the project root for project docs.
- There is no documented `MEMORY.md`-style index: hierarchy affects which instruction files are loaded, not where separate memory bodies are stored.

## Conversion notes for mnemo

Recommended import order:

1. Repo root `AGENTS.md` and nested project `AGENTS.md` files.
2. `AGENTS.override.md` only with explicit opt-in because it may be personal or temporary.
3. `$CODEX_HOME` files only with explicit user-scope import.
4. Configured fallback filenames only when the importer can read Codex configuration safely.

Suggested mapping:

- Build/test commands -> `config`.
- Architecture and workflow conventions -> `pattern`.
- Explicit decisions or rationales -> `decision`.
- Known gotchas -> `discovery`.

Important importer behavior:

- Preserve directory scope so nested instructions do not become global project memory accidentally.
- Record whether a source was an override file.
- Do not infer a native Codex memory database; treat AGENTS.md as prompt-level durable context.

## Sources

- https://openai.com/index/unrolling-the-codex-agent-loop/
- https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/src/agents_md.rs
