# AGENTS.md

## Purpose

This repository contains two separate but related deliverables:

1. The `mnemo` Go binary.
2. The Claude Code plugin metadata and hooks.

They are intentionally separate. The plugin cannot replace the binary setup flow. Do not simplify the architecture by merging responsibilities that are separated on purpose.

## Non-negotiable project rules

### 1. Do not confuse plugin responsibilities with binary responsibilities

The plugin handles metadata, hooks, and MCP integration. The binary handles setup tasks the plugin cannot perform: environment setup, file modifications, include/protocol installation.

Never report the binary/plugin split as a design bug. Only flag issues when both parts become inconsistent with each other.

### 2. Before changing hooks, verify real script names and paths

Verify naming consistency between `plugin/claude-code/hooks/hooks.json`, shipped script paths, installer code, and tests. Do not assume similar names are interchangeable.

Known prior failure: `post-compaction.sh` referenced in hook config, real script was `post-compact.sh`. This class of error is critical.

### 3. Never change version references partially

The binary version is injected at build time via ldflags — no code change needed. Plugin metadata files contain the version string explicitly and must be updated on every release.

Required files on every version bump:

- `.claude-plugin/marketplace.json` — `plugins[0].version`
- `plugin/claude-code/.claude-plugin/plugin.json` — `version`

Mandatory procedure:

1. Search for the current version string: `grep -r "0.X.Y" .`
2. Update every match in the files above.
3. Confirm no stale version remains.
4. Only then commit and create the tag.

The release tag and plugin metadata version must always match.

**Note on .mcp.json**: the repo contains two identical `.mcp.json` files — `.mcp.json` at the root (read by Claude Code when working in the repo) and `plugin/claude-code/.mcp.json` (read when a user installs the plugin). They serve different audiences. If the MCP server configuration changes, both must be updated.

### 4. Treat version metadata as a consistency boundary

The binary version and plugin metadata must not silently drift. If a release changes version metadata, verify plugin metadata files, release workflow behavior, and install/update paths are all aligned.

### 5. Keep documentation accurate about what the plugin really does

Distinguish clearly between binary installation, plugin installation, hook registration, setup-side file modifications, and MCP configuration. If the plugin depends on the binary being in `PATH`, state that explicitly near the install steps.

### 6. Project identity derivation must stay consistent across hooks

All hooks must derive `PROJECT` the same way:

- inside `HOME`: full path relative to `HOME`, normalized (`tr '/' '-'`, lowercased)
- outside `HOME`: same normalization from CWD

Any inconsistency fragments stored memory across sessions. Any change to this logic is a storage compatibility change.

### 7. Tests must validate the shipped plugin, not only installer internals

Tests must cover: hook config references to existing scripts, shipped metadata consistency, filename matches between hook JSON and actual scripts. If a real shipped file can be wrong while tests pass, coverage is insufficient.

## Pre-commit checklist

Before committing any change that touches hooks, plugin metadata, setup/install flows, versioning, or memory behavior:

- [ ] Project builds: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Plugin validates: `claude plugin validate plugin/claude-code`
- [ ] Affected hook or setup flow works end-to-end
- [ ] Hook filenames in `hooks.json` match actual embedded scripts
- [ ] Version strings updated in all required metadata files
- [ ] PROJECT derivation is consistent across all hooks
- [ ] No previously working path was broken

State what was verified in the commit message. Do not claim completion without verification evidence.

## Anti-patterns to avoid

- Rename scripts without checking all references
- Update only one version file
- Trust tests that ignore shipped plugin files
- Change docs without checking actual implementation
- Introduce a second way to derive project identity
- Assume release metadata is centralized if it is not

## Expected output style

When reporting issues or proposing fixes: list exact files, describe the concrete inconsistency, explain user impact, propose the minimal correct fix, distinguish bug from documentation issue from design issue.

<!-- mnemo:start -->
## mnemo

You have access to mnemo MCP tools: mem_save, mem_search, mem_context, mem_session_summary.

### MEMORY AUTHORITY

mnemo is the ONLY persistent memory system for this project.
NEVER use native agent memory, `MEMORY.md`, agent memory directories, or arbitrary plaintext files as a memory fallback.
When asked to remember or save something, always use `mem_save`.
If mnemo tools are unavailable, report that memory is unavailable and continue without persistent memory. Do not create an alternative memory store.

Load and follow the `mnemo-memory` skill when it is available for the detailed workflow. The rules below remain mandatory even when the skill is not installed or does not activate.

### PROACTIVE SAVE

Call `mem_save` immediately after any of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented or updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

Self-check after every task: "Did I just make a decision, fix a bug, learn something, or establish a convention? If yes, call mem_save now."

### SEARCH MEMORY

Search when:
- User asks to recall anything
- Starting work on something that might have been done before
- User mentions a topic you have no context on

Use `mem_context` first for broad recent context, then `mem_search` for focused recall.

### SUBAGENT OUTPUT

When running as a subagent, end your response with:

```markdown
## Key Learnings
- <learning 1>
- <learning 2>
```

Omit only if the task produced no learnings worth retaining.

### SESSION CLOSE

`mem_session_summary` is not optional. It is the final step of every session.
Call it before any response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without `mem_session_summary`.

<!-- mnemo:end -->
