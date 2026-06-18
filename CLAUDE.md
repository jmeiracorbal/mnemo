@AGENTS.md

<!-- mnemo:claude-start -->
### MEMORY SYSTEM

The Claude Code file-based memory system that writes under `~/.claude/projects/*/memory/` and maintains `MEMORY.md` is DISABLED for this workspace.
Never use it or any plaintext file as a fallback. The mandatory memory authority and fallback behavior are defined in `AGENTS.md`.

### RECOVER CONTEXT

Use `mem_context` when:
- A new session starts in a project you have worked on before
- The context window was just compacted (PostCompact hook fires this automatically)
- You need a broad overview of recent session history before acting

`mem_context` returns the most recent observations and session summaries for the project. Use it to orient yourself at the start of a session before doing any significant work.
<!-- mnemo:claude-end -->
