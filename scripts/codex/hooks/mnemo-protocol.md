## mnemo — Persistent Memory Protocol

You have access to mnemo MCP tools: mem_save, mem_search, mem_context, mem_session_summary.

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

### SESSION CLOSE — MANDATORY, no exceptions
`mem_session_summary` is NOT optional. It is the final step of every session.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without `mem_session_summary`.
