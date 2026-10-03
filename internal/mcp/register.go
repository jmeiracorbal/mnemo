package mcp

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func shouldRegister(name string, allowlist map[string]bool) bool {
	if allowlist == nil {
		return true
	}
	return allowlist[name]
}

func registerTools(srv *server.MCPServer, s MemoryBackend, allowlist map[string]bool, runtime *Runtime) {
	// ─── mem_search ─────────────────────────────────────────────────────
	if shouldRegister("mem_search", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_search",
				mcp.WithDescription(strings.TrimSpace(memSearchDescription)),
				mcp.WithTitleAnnotation("Search Memory"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("query",
					mcp.Description("Search query — natural language or keywords. Optional: omit to browse by tags, topic_key, or other filters."),
				),
				mcp.WithString("type",
					mcp.Description("Filter by type: tool_use, file_change, command, file_read, search, manual, decision, architecture, bugfix, pattern"),
				),
				mcp.WithString("project",
					mcp.Description("Filter by project name"),
				),
				mcp.WithString("scope",
					mcp.Description("Filter by scope: project (default) or personal"),
				),
				mcp.WithNumber("limit",
					mcp.Description("Max results (default: 10, max: 20)"),
				),
				mcp.WithString("tags",
					mcp.Description("Comma-separated tags to filter by (e.g. \"auth,backend\"). Only observations with ALL listed tags are returned."),
				),
				mcp.WithString("prefer_tags",
					mcp.Description("Comma-separated tags for soft ranking (e.g. \"auth,backend\"). Observations matching more of these tags rank higher; non-matching observations are still included."),
				),
				mcp.WithString("topic_key",
					mcp.Description("Filter by topic key (e.g. \"auth/jwt-middleware\"). Only observations with this exact topic_key are returned."),
				),
			),
			handleSearch(s),
		)
	}

	// ─── mem_save ───────────────────────────────────────────────────────
	if shouldRegister("mem_save", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_save",
				mcp.WithTitleAnnotation("Save Memory"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(false),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithDescription(strings.TrimSpace(memSaveDescription)),
				mcp.WithString("title",
					mcp.Required(),
					mcp.Description("Short, searchable title (e.g. 'JWT auth middleware', 'Fixed N+1 query')"),
				),
				mcp.WithString("content",
					mcp.Required(),
					mcp.Description("Structured content using **What**, **Why**, **Where**, **Learned** format"),
				),
				mcp.WithString("type",
					mcp.Description("Category: decision, architecture, bugfix, pattern, config, discovery, learning (default: manual)"),
				),
				mcp.WithString("project",
					mcp.Required(),
					mcp.Description("Project name"),
				),
				mcp.WithString("directory",
					mcp.Required(),
					mcp.Description("Project working directory"),
				),
				mcp.WithString("scope",
					mcp.Description("Scope for this observation: project (default) or personal"),
				),
				mcp.WithString("topic_key",
					mcp.Description("Optional topic identifier for upserts (e.g. architecture/auth-model). Reuses and updates the latest observation in same project+scope."),
				),
				mcp.WithString("tags",
					mcp.Description("Comma-separated tags to attach (e.g. \"auth,backend,decision\")."),
				),
			),
			handleSave(s, runtime),
		)
	}

	// ─── mem_update (deferred) ──────────────────────────────────────────
	if shouldRegister("mem_update", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_update",
				mcp.WithDescription("Update an existing observation by ID. Only provided fields are changed."),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Update Memory"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(false),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithNumber("id",
					mcp.Required(),
					mcp.Description("Observation ID to update"),
				),
				mcp.WithString("title",
					mcp.Description("New title"),
				),
				mcp.WithString("content",
					mcp.Description("New content"),
				),
				mcp.WithString("type",
					mcp.Description("New type/category"),
				),
				mcp.WithString("project",
					mcp.Description("New project value"),
				),
				mcp.WithString("scope",
					mcp.Description("New scope: project or personal"),
				),
				mcp.WithString("topic_key",
					mcp.Description("New topic key (normalized internally)"),
				),
				mcp.WithString("tags",
					mcp.Description("Comma-separated replacement tags. Empty string removes all tags. Omitting this field leaves tags unchanged."),
				),
			),
			handleUpdate(s),
		)
	}

	// ─── mem_suggest_topic_key (deferred) ───────────────────────────────
	if shouldRegister("mem_suggest_topic_key", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_suggest_topic_key",
				mcp.WithDescription(strings.TrimSpace(memSuggestTopicKeyDescription)),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Suggest Topic Key"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("type",
					mcp.Description("Observation type/category, e.g. architecture, decision, bugfix"),
				),
				mcp.WithString("title",
					mcp.Description("Observation title (preferred input for stable keys)"),
				),
				mcp.WithString("content",
					mcp.Description("Observation content used as fallback if title is empty"),
				),
			),
			handleSuggestTopicKey(),
		)
	}

	// ─── mem_delete (admin, deferred) ───────────────────────────────────
	if shouldRegister("mem_delete", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_delete",
				mcp.WithDescription("Delete an observation by ID. mnemo preserves decisions by using logical deletion; permanent deletion is not exposed through MCP."),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Delete Memory"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(true),
				mcp.WithIdempotentHintAnnotation(false),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithNumber("id",
					mcp.Required(),
					mcp.Description("Observation ID to delete"),
				),
			),
			handleDelete(s),
		)
	}

	// ─── mem_save_prompt (deferred) ─────────────────────────────────────
	if shouldRegister("mem_save_prompt", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_save_prompt",
				mcp.WithDescription(strings.TrimSpace(memSavePromptDescription)),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Save User Prompt"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(false),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("content",
					mcp.Required(),
					mcp.Description("The user's prompt text"),
				),
				mcp.WithString("project",
					mcp.Required(),
					mcp.Description("Project name"),
				),
				mcp.WithString("directory",
					mcp.Required(),
					mcp.Description("Project working directory"),
				),
			),
			handleSavePrompt(s, runtime),
		)
	}

	// ─── mem_context ────────────────────────────────────────────────────
	if shouldRegister("mem_context", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_context",
				mcp.WithDescription(strings.TrimSpace(memContextDescription)),
				mcp.WithTitleAnnotation("Get Memory Context"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("project",
					mcp.Description("Filter by project (omit for all projects)"),
				),
				mcp.WithString("scope",
					mcp.Description("Filter observations by scope: project (default) or personal"),
				),
				mcp.WithNumber("limit",
					mcp.Description("Number of observations to retrieve (default: 20)"),
				),
				mcp.WithString("tags",
					mcp.Description("Comma-separated tags to filter observations (e.g. \"auth,backend\"). Only observations with ALL listed tags are included."),
				),
				mcp.WithString("prefer_tags",
					mcp.Description("Comma-separated tags for soft ranking. Observations matching more of these tags are surfaced first."),
				),
				mcp.WithString("topic_key",
					mcp.Description("Topic key to prioritize (e.g. \"auth/jwt-middleware\"). Observations with this topic_key are ranked first; others are still included."),
				),
			),
			handleContext(s),
		)
	}

	// ─── mem_list_tags ───────────────────────────────────────────────────
	if shouldRegister("mem_list_tags", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_list_tags",
				mcp.WithDescription(strings.TrimSpace(memListTagsDescription)),
				mcp.WithTitleAnnotation("List Tags"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("project",
					mcp.Description("Project name (omit for all projects)"),
				),
			),
			handleListTags(s),
		)
	}

	// ─── mem_merge_tags ─────────────────────────────────────────────────
	if shouldRegister("mem_merge_tags", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_merge_tags",
				mcp.WithDescription(strings.TrimSpace(memMergeTagsDescription)),
				mcp.WithTitleAnnotation("Merge Tags"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("from",
					mcp.Required(),
					mcp.Description("Tag to merge away (source). Will be removed after merging."),
				),
				mcp.WithString("to",
					mcp.Required(),
					mcp.Description("Target canonical tag. Must not be a blocked/generic tag."),
				),
			),
			handleMergeTags(s),
		)
	}

	// ─── mem_tag_stats ──────────────────────────────────────────────────
	if shouldRegister("mem_tag_stats", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_tag_stats",
				mcp.WithDescription(strings.TrimSpace(memTagStatsDescription)),
				mcp.WithTitleAnnotation("Tag Stats"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("project",
					mcp.Description("Project name (omit for all projects)"),
				),
				mcp.WithNumber("min_count",
					mcp.Description("Only include tags used at least this many times (hard filter, 0 = no lower bound)"),
				),
				mcp.WithNumber("max_count",
					mcp.Description("Only include tags used at most this many times — use to surface low-frequency tags (0 = no upper bound)"),
				),
				mcp.WithString("unused_since",
					mcp.Description("ISO8601 date. Only include tags not used since this date — use to surface stale tags (e.g. '2026-01-01T00:00:00Z')"),
				),
				mcp.WithNumber("limit",
					mcp.Description("Max results to return (default: 20, 0 = no limit)"),
				),
				mcp.WithString("sort_by",
					mcp.Description("Result ordering: 'freq' (highest frequency first, default), 'stale' (oldest last-used first), 'alpha' (alphabetical)"),
				),
			),
			handleTagStats(s),
		)
	}

	// ─── mem_related_tags ───────────────────────────────────────────────
	if shouldRegister("mem_related_tags", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_related_tags",
				mcp.WithDescription(strings.TrimSpace(memRelatedTagsDescription)),
				mcp.WithTitleAnnotation("Related Tags"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("tag",
					mcp.Required(),
					mcp.Description("Tag to find related tags for."),
				),
				mcp.WithString("project",
					mcp.Description("Project name (omit for all projects)."),
				),
				mcp.WithNumber("limit",
					mcp.Description("Max results to return (default: 20, 0 = no limit)."),
				),
				mcp.WithString("since",
					mcp.Description("ISO8601 date. Only count co-occurrences after this date (e.g. '2026-01-01')."),
				),
				mcp.WithNumber("min_cooccurrence",
					mcp.Description("Minimum co-occurrence count to include a tag (default: 1)."),
				),
				mcp.WithBoolean("include_observations",
					mcp.Description("Include co-occurrences from observations (default: true)."),
				),
				mcp.WithBoolean("include_sessions",
					mcp.Description("Include co-occurrences from sessions (default: true)."),
				),
			),
			handleRelatedTags(s),
		)
	}

	// ─── mem_stats (admin, deferred) ────────────────────────────────────
	if shouldRegister("mem_stats", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_stats",
				mcp.WithDescription("Show memory system statistics — total sessions, observations, and projects tracked."),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Memory Stats"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
			),
			handleStats(s),
		)
	}

	// ─── mem_timeline (admin, deferred) ─────────────────────────────────
	if shouldRegister("mem_timeline", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_timeline",
				mcp.WithDescription(strings.TrimSpace(memTimelineDescription)),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Memory Timeline"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithNumber("observation_id",
					mcp.Required(),
					mcp.Description("The observation ID to center the timeline on (from mem_search results)"),
				),
				mcp.WithNumber("before",
					mcp.Description("Number of observations to show before the focus (default: 5)"),
				),
				mcp.WithNumber("after",
					mcp.Description("Number of observations to show after the focus (default: 5)"),
				),
			),
			handleTimeline(s),
		)
	}

	// ─── mem_get_observation (deferred) ─────────────────────────────────
	if shouldRegister("mem_get_observation", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_get_observation",
				mcp.WithDescription(strings.TrimSpace(memGetObservationDescription)),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Get Observation"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithNumber("id",
					mcp.Required(),
					mcp.Description("The observation ID to retrieve"),
				),
			),
			handleGetObservation(s),
		)
	}

	// ─── mem_session_summary ────────────────────────────────────────────
	if shouldRegister("mem_session_summary", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_session_summary",
				mcp.WithTitleAnnotation("Save Session Summary"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(false),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithDescription(strings.TrimSpace(memSessionSummaryDescription)),
				mcp.WithString("content",
					mcp.Required(),
					mcp.Description("Full session summary using the Goal/Instructions/Discoveries/Accomplished/Files format"),
				),
				mcp.WithString("project",
					mcp.Required(),
					mcp.Description("Unique project identifier"),
				),
				mcp.WithString("directory",
					mcp.Required(),
					mcp.Description("Project working directory"),
				),
			),
			handleSessionSummary(s, runtime),
		)
	}

	// ─── mem_capture_passive (deferred) ─────────────────────────────────
	if shouldRegister("mem_capture_passive", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_capture_passive",
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Capture Learnings"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithDescription(strings.TrimSpace(memCapturePassiveDescription)),
				mcp.WithString("content",
					mcp.Required(),
					mcp.Description("The text output containing a '## Key Learnings:' section with numbered or bulleted items"),
				),
				mcp.WithString("project",
					mcp.Required(),
					mcp.Description("Project name"),
				),
				mcp.WithString("directory",
					mcp.Required(),
					mcp.Description("Project working directory"),
				),
				mcp.WithString("source",
					mcp.Description("Source identifier (e.g. 'subagent-stop', 'session-end')"),
				),
			),
			handleCapturePassive(s, runtime),
		)
	}

	registerDiagnosticTools(srv, allowlist)
}
