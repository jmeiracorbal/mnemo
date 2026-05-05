## mnemo — Persistent Memory Protocol

You have access to mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).

### MEMORY SYSTEM — mnemo is the ONLY memory system
**NEVER use the file-based memory system** (the one that writes `.md` files to `~/.claude/projects/*/memory/` and maintains a `MEMORY.md` index). That system is DISABLED for this workspace.
When asked to "save to memory", "remember this", or "guardar en memoria" — ALWAYS use `mem_save`. Never write files.

### PROACTIVE SAVE — do NOT wait for user to ask
Call `mem_save` IMMEDIATELY after ANY of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented/updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

**Self-check after EVERY task**: "Did I just make a decision, fix a bug, learn something, or establish a convention? If yes → mem_save NOW."

### SEARCH MEMORY when:
- User asks to recall anything
- Starting work on something that might have been done before
- User mentions a topic you have no context on

### RECOVER CONTEXT with mem_context when:
- A new session starts in a project you have worked on before
- The context window was just compacted (PostCompact hook fires this automatically)
- You need a broad overview of recent session history before acting

`mem_context` returns the most recent observations and session summaries for the project. Use it to orient yourself at the start of a session before doing any significant work.

### SUBAGENT OUTPUT — required format for passive capture
When running as a subagent, always end your response with a structured section:

```
## Key Learnings
- <learning 1>
- <learning 2>
```

This enables mnemo to automatically extract and persist what you discovered.
Omit the section only if the task produced no learnings worth retaining.

### SESSION CLOSE — MANDATORY, no exceptions
`mem_session_summary` is NOT optional. It is the final step of every session, like a `defer` — it always runs.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without `mem_session_summary`.
