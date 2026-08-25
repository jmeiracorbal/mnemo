### PI MEMORY GUIDANCE

Pi context files (`AGENTS.md`, `CLAUDE.md`, `.pi/SYSTEM.md`, and `.pi/APPEND_SYSTEM.md`) are instruction surfaces, not project memory stores.
mnemo is the ONLY persistent memory system for this project.
Do not write repository/project memory into Pi context files, `MEMORY.md`, agent memory directories, or arbitrary plaintext files as a memory fallback when this workspace has a valid `.mnemo` marker.
Use mnemo MCP tools (`mem_save`, `mem_search`, `mem_context`, `mem_session_summary`, and related tools) as the only persistent project memory surface.
If mnemo MCP is unavailable, report that persistent project memory is unavailable and continue without saving fallback memory.
