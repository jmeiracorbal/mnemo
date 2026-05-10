### MEMORY SYSTEM

**NEVER use the file-based memory system** (the one that writes `.md` files to `~/.claude/projects/*/memory/` and maintains a `MEMORY.md` index). That system is DISABLED for this workspace.
When asked to "save to memory", "remember this", or "guardar en memoria": always use `mem_save`. Never write files.

### RECOVER CONTEXT

Use `mem_context` when:
- A new session starts in a project you have worked on before
- The context window was just compacted (PostCompact hook fires this automatically)
- You need a broad overview of recent session history before acting

`mem_context` returns the most recent observations and session summaries for the project. Use it to orient yourself at the start of a session before doing any significant work.
