---
name: mnemo-memory
description: ALWAYS ACTIVE when .mnemo exists — mnemo is the ONLY persistent memory for initialized projects. Never use MEMORY.md or native agent memory. Use for session start, compaction recovery, recalls, saves, and session close. Verify a valid .mnemo marker first.
---

# mnemo Memory

Use mnemo MCP tools to recover and persist project knowledge across sessions. Keep the always-active project instructions authoritative; this skill provides the detailed workflow.

## 1. Verify the project

Before any memory operation:

1. Resolve the Git repository root, or use the current workspace when it is not a Git repository.
2. Read `<root>/.mnemo`.
3. Continue only when it is valid JSON with a non-empty `id`.
4. Use that `id` as `project` in every mnemo tool call.

If `.mnemo` is missing or invalid, tell the user to run `mnemo init` and stop the memory workflow. If mnemo tools are unavailable, report that integration is incomplete. Never create `MEMORY.md`, write into an agent's native memory directory, or use arbitrary text files as a fallback.

### Troubleshoot integration

When mnemo appears misconfigured, unavailable, or inconsistent with the project marker, prefer the read-only diagnostic command before guessing:

```bash
mnemo doctor --path <root>
mnemo doctor --json --path <root>
```

Use the output to explain missing `.mnemo`, PATH, global instruction, MCP, hook/plugin, or store issues. Do not repair global setup automatically unless the user explicitly asks for setup or install changes.

### Database upgrade recovery

mnemo applies safe database migrations automatically when any CLI, MCP, or hook path opens the store. If an older local database is detected, the normal agent workflow should recover it without a separate user decision.

If a mnemo command or MCP tool reports that the database requires migration, is dirty, is ahead of the bundled migrations, or has an inconsistent schema:

1. Run `mnemo db migrate --check` to inspect the state without writing.
2. If the state only shows pending bundled migrations, run `mnemo db migrate` and retry the original mnemo action.
3. If the state is dirty, ahead, or inconsistent, stop and report the exact diagnostic instead of editing SQLite manually.

Never add ad hoc DDL or direct SQLite repairs in an agent workflow; schema changes must come from bundled mnemo migrations.

### Version update notice

Do not treat this skill as the update mechanism. Released mnemo binaries own
version detection, confirmation, download and installation.

When an interactive mnemo CLI call prints `[mnemo] update available`, tell the
user the installed and latest versions. If mnemo prompts `Update now? [y/N]`,
let the user answer the prompt. If the command is not currently prompting and
the user explicitly approves an update, run `mnemo update` or
`mnemo update --yes --agent=all`, then retry the original mnemo command or
diagnostic.

Do not update mnemo silently, and do not run update checks from MCP, hooks, or
JSON-output paths.

## 2. Recover relevant context

- At session start, resume, or after compaction, call `mem_context` before significant work.
- When the user asks to recall past work, call `mem_context` first, then `mem_search` with focused keywords.
- Use `mem_get_observation` when a search result is truncated or the full record matters.
- Search proactively when beginning work that may have prior decisions or when an unfamiliar topic may have been discussed before.

After compaction, persist the compacted summary with `mem_session_summary` before recovering context if the active hook instructions require it.

## 3. Save important knowledge

Call `mem_save` immediately after:

- architecture or design decisions;
- completed bug fixes, including root cause;
- conventions, workflows, or configuration changes;
- non-obvious discoveries and edge cases;
- stable user preferences or constraints;
- meaningful file-structure or integration changes.

Use:

- a short searchable title;
- the most specific type available;
- structured `What`, `Why`, `Where`, and optional `Learned` content;
- concise, relevant tags;
- `scope=project` unless the memory genuinely applies across projects.

For an evolving topic, call `mem_suggest_topic_key` and reuse the returned key. Use `mem_update` only when correcting a known observation by ID. Do not save routine progress, guesses, or information already obvious from the code.

When the user explicitly asks to remember something, always use `mem_save`.

## 4. Capture delegated work

When acting as a subagent, end useful output with:

```markdown
## Key Learnings
- <durable learning>
```

Omit the section only when no durable learning was produced. This supports passive capture by mnemo hooks.

## 5. Close the session

Before any response that signals completion or goodbye, call `mem_session_summary` with:

```markdown
## Goal
<session objective>

## Instructions
<stable user constraints, if any>

## Discoveries
- <non-obvious finding>

## Accomplished
- <completed work>

## Next Steps
- <remaining work>

## Relevant Files
- <path and role>
```

Keep the summary concise but sufficient for another agent to resume the work.
