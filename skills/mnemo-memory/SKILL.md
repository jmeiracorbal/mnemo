---
name: mnemo-memory
description: Use mnemo as the only persistent memory for an initialized project. Use when starting or resuming work, recovering context after compaction, recalling prior decisions, saving important decisions or fixes, recording conventions or user preferences, and closing a task or session. Always verify a valid .mnemo marker first; never fall back to native, file-based, or plaintext memory.
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
