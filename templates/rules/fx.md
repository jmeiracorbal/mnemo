### FX NATIVE MEMORY

fx's built-in `memory` tool and `~/.fx/memories.json` are DISABLED for repository/project memory when this workspace has a valid `.mnemo` marker.
Do not call the native `memory` tool to save, list, clear, remember, forget, or recall project facts, decisions, bugs, conventions, session summaries, or implementation notes.
Use mnemo MCP tools (`mem_save`, `mem_search`, `mem_context`, `mem_session_summary`, and related tools) as the only persistent project memory surface.
If mnemo MCP is unavailable, report that persistent project memory is unavailable and continue without saving to fx native memory.
