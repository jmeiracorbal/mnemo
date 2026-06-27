## mnemo — Persistent Memory Protocol

You have access to mnemo MCP tools. Key tools: mem_save, mem_search, mem_context, mem_session_summary, mem_update, mem_get_observation, mem_session_end, mem_suggest_topic_key.

### MEMORY AUTHORITY
mnemo is the ONLY persistent memory system for this project.
NEVER use native agent memory, `MEMORY.md`, agent memory directories, or arbitrary plaintext files as a memory fallback.
When asked to remember or save something, always use `mem_save`.
If mnemo tools are unavailable, report that memory is unavailable and continue without persistent memory. Do not create an alternative memory store.

Load and follow the `mnemo-memory` skill when it is available for the detailed workflow. These rules remain mandatory even when the skill is not installed or does not activate.

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

Use `mem_context` first for broad recent context, then `mem_search` for focused recall.

### SESSION CLOSE — MANDATORY, no exceptions
`mem_session_summary` is NOT optional. It is the final step of every session.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without `mem_session_summary`.
