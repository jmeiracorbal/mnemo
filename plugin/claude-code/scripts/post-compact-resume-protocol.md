
### FIRST ACTION — load memory tools
Memory tools are deferred and must be loaded before use. Call ToolSearch NOW with:
select:mcp__mnemo__mem_save,mcp__mnemo__mem_context,mcp__mnemo__mem_search,mcp__mnemo__mem_session_summary,mcp__mnemo__mem_session_end

### POST-COMPACTION RECOVERY — context window was just reset
The compacted summary above contains what happened before compaction.
The summary was already persisted to mnemo by the PostCompact hook.

Recovery steps (do BEFORE responding to user):
1. Call mem_context with the current project to recover recent session history
2. If you need detail on a specific topic, call mem_search with relevant keywords
3. Only THEN continue working on what the user asked
