// Package mcp implements the Model Context Protocol server for mnemo.
//
// This exposes memory tools via MCP stdio transport so any agent
// (Claude Code, OpenCode, Gemini CLI, Codex, etc.) can use mnemo's
// persistent memory just by adding it as an MCP server.
//
// Tool profiles allow agents to load only the tools they need:
//
//	mnemo mcp                    → all tools (default)
//	mnemo mcp --tools=agent      → 11 tools agents actually use
//	mnemo mcp --tools=admin      → 3 tools for CLI curation (delete, stats, timeline)
//	mnemo mcp --tools=mem_save,mem_search → individual tool names
package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var suggestTopicKey = store.SuggestTopicKey
var suggestTags = store.SuggestTags

// parseTags splits a comma-separated tag string and returns individual values.
// Empty strings and blank tokens are dropped.
func parseTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

var loadMCPStats = func(s *store.Store) (*store.Stats, error) {
	return s.Stats()
}

// ─── Tool Profiles ───────────────────────────────────────────────────────────

// ProfileAgent contains the tool names that AI agents need during coding sessions.
var ProfileAgent = map[string]bool{
	"mem_save":              true,
	"mem_search":            true,
	"mem_context":           true,
	"mem_session_summary":   true,
	"mem_session_start":     true,
	"mem_session_end":       true,
	"mem_get_observation":   true,
	"mem_suggest_topic_key": true,
	"mem_capture_passive":   true,
	"mem_save_prompt":       true,
	"mem_update":            true,
	"mem_list_tags":         true,
	"mem_merge_tags":        true,
	"mem_tag_stats":         true,
	"mem_related_tags":      true,
}

// ProfileAdmin contains tools for CLI curation and dashboards.
var ProfileAdmin = map[string]bool{
	"mem_delete":   true,
	"mem_stats":    true,
	"mem_timeline": true,
}

// Profiles maps profile names to their tool sets.
var Profiles = map[string]map[string]bool{
	"agent": ProfileAgent,
	"admin": ProfileAdmin,
}

// ResolveTools takes a comma-separated string of profile names and/or
// individual tool names and returns the set of tool names to register.
// An empty input means "all" — every tool is registered.
func ResolveTools(input string) map[string]bool {
	input = strings.TrimSpace(input)
	if input == "" || input == "all" {
		return nil
	}

	result := make(map[string]bool)
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == "all" {
			return nil
		}
		if profile, ok := Profiles[token]; ok {
			for tool := range profile {
				result[tool] = true
			}
		} else {
			result[token] = true
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// NewServer creates an MCP server with ALL tools registered.
func NewServer(s *store.Store) *server.MCPServer {
	return NewServerWithTools(s, nil)
}

const serverInstructions = `mnemo provides persistent memory that survives across sessions and context ` +
	`compactions. Search these tools when you need to: save decisions, bugs, ` +
	`architecture choices, or discoveries to memory; recall or search past work ` +
	`from previous sessions; manage coding session lifecycle (start, end, ` +
	`summarize); recover context after compaction. Key tools: mem_save, ` +
	`mem_search, mem_context, mem_session_summary, mem_get_observation, ` +
	`mem_suggest_topic_key.`

// NewServerWithTools creates an MCP server registering only the tools in
// the allowlist. If allowlist is nil, all tools are registered.
func NewServerWithTools(s *store.Store, allowlist map[string]bool) *server.MCPServer {
	srv := server.NewMCPServer(
		"mnemo",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(serverInstructions),
	)

	registerTools(srv, s, allowlist)
	return srv
}

func shouldRegister(name string, allowlist map[string]bool) bool {
	if allowlist == nil {
		return true
	}
	return allowlist[name]
}

func registerTools(srv *server.MCPServer, s *store.Store, allowlist map[string]bool) {
	// ─── mem_search ─────────────────────────────────────────────────────
	if shouldRegister("mem_search", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_search",
				mcp.WithDescription("Search your persistent memory across all sessions. Use this to find past decisions, bugs fixed, patterns used, files changed, or any context from previous coding sessions."),
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
				mcp.WithDescription(`Save an important observation to persistent memory. Call this PROACTIVELY after completing significant work — don't wait to be asked.

WHEN to save (call this after each of these):
- Architectural decisions or tradeoffs
- Bug fixes (what was wrong, why, how you fixed it)
- New patterns or conventions established
- Configuration changes or environment setup
- Important discoveries or gotchas
- File structure changes

FORMAT for content — use this structured format:
  **What**: [concise description of what was done]
  **Why**: [the reasoning, user request, or problem that drove it]
  **Where**: [files/paths affected, e.g. src/auth/middleware.ts, internal/store/store.go]
  **Learned**: [any gotchas, edge cases, or decisions made — omit if none]

TITLE should be short and searchable, like: "JWT auth middleware", "FTS5 query sanitization", "Fixed N+1 in user list"

Examples:
  title: "Switched from sessions to JWT"
  type: "decision"
  content: "**What**: Replaced express-session with jsonwebtoken for auth\n**Why**: Session storage doesn't scale across multiple instances\n**Where**: src/middleware/auth.ts, src/routes/login.ts\n**Learned**: Must set httpOnly and secure flags on the cookie, refresh tokens need separate rotation logic"

  title: "Fixed FTS5 syntax error on special chars"
  type: "bugfix"
  content: "**What**: Wrapped each search term in quotes before passing to FTS5 MATCH\n**Why**: Users typing queries like 'fix auth bug' would crash because FTS5 interprets special chars as operators\n**Where**: internal/store/store.go — sanitizeFTS() function\n**Learned**: FTS5 MATCH syntax is NOT the same as LIKE — always sanitize user input"`),
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
				mcp.WithString("session_id",
					mcp.Description("Session ID to associate with (default: manual-save-{project})"),
				),
				mcp.WithString("project",
					mcp.Description("Project name"),
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
			handleSave(s),
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
				mcp.WithDescription("Suggest a stable topic_key for memory upserts. Use this before mem_save when you want evolving topics (like architecture decisions) to update a single observation over time."),
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
				mcp.WithDescription("Delete an observation by ID. Soft-delete by default; set hard_delete=true for permanent deletion."),
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
				mcp.WithBoolean("hard_delete",
					mcp.Description("If true, permanently deletes the observation"),
				),
			),
			handleDelete(s),
		)
	}

	// ─── mem_save_prompt (deferred) ─────────────────────────────────────
	if shouldRegister("mem_save_prompt", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_save_prompt",
				mcp.WithDescription("Save a user prompt to persistent memory. Use this to record what the user asked — their intent, questions, and requests — so future sessions have context about the user's goals."),
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
				mcp.WithString("session_id",
					mcp.Description("Session ID to associate with (default: manual-save-{project})"),
				),
				mcp.WithString("project",
					mcp.Description("Project name"),
				),
			),
			handleSavePrompt(s),
		)
	}

	// ─── mem_context ────────────────────────────────────────────────────
	if shouldRegister("mem_context", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_context",
				mcp.WithDescription("Get recent memory context from previous sessions. Shows recent sessions and observations to understand what was done before."),
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
				mcp.WithDescription("List all tags in use for a project, ordered by frequency. Use this to discover the tag vocabulary before filtering or searching."),
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
				mcp.WithDescription("Merge all occurrences of one tag into another. Use to consolidate aliases or plural/canonical variants (e.g. 'authentication' → 'auth'). The source tag is removed after merging."),
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
				mcp.WithDescription("Query tag observability for a project: top tags, low-frequency tags, or stale tags not used recently. Use min_count/max_count/unused_since as hard filters and sort_by to control ordering."),
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
				mcp.WithDescription("Find tags that frequently co-occur with a given tag in observations or sessions. Useful for discovering related topics and navigating the tag graph."),
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
				mcp.WithDescription("Show chronological context around a specific observation. Use after mem_search to drill into the timeline of events surrounding a search result."),
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
				mcp.WithDescription("Get the full content of a specific observation by ID. Use when you need the complete, untruncated content of an observation found via mem_search or mem_timeline."),
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
				mcp.WithDescription(`Save a comprehensive end-of-session summary. Call this when a session is ending or when significant work is complete. This creates a structured summary that future sessions will use to understand what happened.

FORMAT — use this exact structure in the content field:

## Goal
[One sentence: what were we building/working on in this session]

## Instructions
[User preferences, constraints, or context discovered during this session. Things a future agent needs to know about HOW the user wants things done. Skip if nothing notable.]

## Discoveries
- [Technical finding, gotcha, or learning 1]
- [Technical finding 2]
- [Important API behavior, config quirk, etc.]

## Accomplished
- ✅ [Completed task 1 — with key implementation details]
- ✅ [Completed task 2 — mention files changed]
- 🔲 [Identified but not yet done — for next session]

## Relevant Files
- path/to/file.ts — [what it does or what changed]
- path/to/other.go — [role in the architecture]

GUIDELINES:
- Be CONCISE but don't lose important details (file paths, error messages, decisions)
- Focus on WHAT and WHY, not HOW (the code itself is in the repo)
- Include things that would save a future agent time
- The Discoveries section is the most valuable — capture gotchas and non-obvious learnings
- Relevant Files should only include files that were significantly changed or are important for context`),
				mcp.WithString("content",
					mcp.Required(),
					mcp.Description("Full session summary using the Goal/Instructions/Discoveries/Accomplished/Files format"),
				),
				mcp.WithString("session_id",
					mcp.Description("Session ID (default: manual-save-{project})"),
				),
				mcp.WithString("project",
					mcp.Required(),
					mcp.Description("Project name"),
				),
			),
			handleSessionSummary(s),
		)
	}

	// ─── mem_session_start (deferred) ───────────────────────────────────
	if shouldRegister("mem_session_start", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_session_start",
				mcp.WithDescription("Register the start of a new coding session. Call this at the beginning of a session to track activity."),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("Start Session"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("id",
					mcp.Required(),
					mcp.Description("Unique session identifier"),
				),
				mcp.WithString("project",
					mcp.Required(),
					mcp.Description("Project name"),
				),
				mcp.WithString("directory",
					mcp.Description("Working directory"),
				),
			),
			handleSessionStart(s),
		)
	}

	// ─── mem_session_end (deferred) ─────────────────────────────────────
	if shouldRegister("mem_session_end", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_session_end",
				mcp.WithDescription("Mark a coding session as completed with an optional summary."),
				mcp.WithDeferLoading(true),
				mcp.WithTitleAnnotation("End Session"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("id",
					mcp.Required(),
					mcp.Description("Session identifier to close"),
				),
				mcp.WithString("summary",
					mcp.Description("Summary of what was accomplished"),
				),
				mcp.WithString("tags",
					mcp.Description("Comma-separated tags for this session (e.g. 'feature,auth,backend')"),
				),
			),
			handleSessionEnd(s),
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
				mcp.WithDescription(`Extract and save structured learnings from text output. Use this at the end of a task to capture knowledge automatically.

The tool looks for sections like "## Key Learnings:" or "## Aprendizajes Clave:" and extracts numbered or bulleted items. Each item is saved as a separate observation.

Duplicates are automatically detected and skipped — safe to call multiple times with the same content.`),
				mcp.WithString("content",
					mcp.Required(),
					mcp.Description("The text output containing a '## Key Learnings:' section with numbered or bulleted items"),
				),
				mcp.WithString("session_id",
					mcp.Description("Session ID (default: manual-save-{project})"),
				),
				mcp.WithString("project",
					mcp.Description("Project name"),
				),
				mcp.WithString("source",
					mcp.Description("Source identifier (e.g. 'subagent-stop', 'session-end')"),
				),
			),
			handleCapturePassive(s),
		)
	}
}

// ─── Tool Handlers ───────────────────────────────────────────────────────────

func handleSearch(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, _ := req.GetArguments()["query"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		project, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		limit := intArg(req, "limit", 10)
		tagsRaw, _ := req.GetArguments()["tags"].(string)
		boostTagsRaw, _ := req.GetArguments()["prefer_tags"].(string)

		results, err := s.Search(query, store.SearchOptions{
			Type:      typ,
			Project:   project,
			Scope:     scope,
			TopicKey:  topicKey,
			Tags:      parseTags(tagsRaw),
			PreferTags: parseTags(boostTagsRaw),
			Limit:     limit,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search error: %s. Try simpler keywords.", err)), nil
		}

		if len(results) == 0 {
			if query == "" {
				return mcp.NewToolResultText("No memories found matching the specified filters."), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("No memories found for: %q", query)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d memories:\n\n", len(results))
		anyTruncated := false
		for i, r := range results {
			proj := ""
			if r.Project != nil {
				proj = fmt.Sprintf(" | project: %s", *r.Project)
			}
			preview := truncate(r.Content, 300)
			if len(r.Content) > 300 {
				anyTruncated = true
				preview += " [truncated — use mem_get_observation(id) for full content]"
			}
			fmt.Fprintf(&b, "[%d] #%d (%s) — %s\n    %s\n    %s%s | scope: %s\n\n",
				i+1, r.ID, r.Type, r.Title,
				preview,
				r.CreatedAt, proj, r.Scope)
		}
		if anyTruncated {
			fmt.Fprintf(&b, "---\nResults above are previews (300 chars). To read the full content of a specific memory, call mem_get_observation(id: <ID>).\n")
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

func handleSave(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, _ := req.GetArguments()["title"].(string)
		content, _ := req.GetArguments()["content"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		project, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		tagsRaw, _ := req.GetArguments()["tags"].(string)
		tags := parseTags(tagsRaw)

		if typ == "" {
			typ = "manual"
		}
		if sessionID == "" {
			sessionID = defaultSessionID(project)
		}
		suggestedTopicKey := suggestTopicKey(typ, title, content)

		if err := s.CreateSession(sessionID, project, ""); err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}

		truncated := len(content) > s.MaxObservationLength()

		_, err := s.AddObservation(store.AddObservationParams{
			SessionID: sessionID,
			Type:      typ,
			Title:     title,
			Content:   content,
			Project:   project,
			Scope:     scope,
			TopicKey:  topicKey,
			Tags:      tags,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Memory saved: %q (%s)", title, typ)
		if topicKey == "" && suggestedTopicKey != "" {
			msg += fmt.Sprintf("\nSuggested topic_key: %s", suggestedTopicKey)
		}
		if len(tags) == 0 {
			if suggested := suggestTags(typ, title, content); len(suggested) > 0 {
				msg += fmt.Sprintf("\nSuggested tags: %s", strings.Join(suggested, ", "))
			}
		}
		if truncated {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", len(content), s.MaxObservationLength())
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleSuggestTopicKey() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typ, _ := req.GetArguments()["type"].(string)
		title, _ := req.GetArguments()["title"].(string)
		content, _ := req.GetArguments()["content"].(string)

		if strings.TrimSpace(title) == "" && strings.TrimSpace(content) == "" {
			return mcp.NewToolResultError("provide title or content to suggest a topic_key"), nil
		}

		topicKey := suggestTopicKey(typ, title, content)
		if topicKey == "" {
			return mcp.NewToolResultError("could not suggest topic_key from input"), nil
		}

		msg := fmt.Sprintf("Suggested topic_key: %s", topicKey)
		if tags := suggestTags(typ, title, content); len(tags) > 0 {
			msg += fmt.Sprintf("\nSuggested tags: %s", strings.Join(tags, ", "))
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleUpdate(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		update := store.UpdateObservationParams{}
		if v, ok := req.GetArguments()["title"].(string); ok {
			update.Title = &v
		}
		if v, ok := req.GetArguments()["content"].(string); ok {
			update.Content = &v
		}
		if v, ok := req.GetArguments()["type"].(string); ok {
			update.Type = &v
		}
		if v, ok := req.GetArguments()["project"].(string); ok {
			update.Project = &v
		}
		if v, ok := req.GetArguments()["scope"].(string); ok {
			update.Scope = &v
		}
		if v, ok := req.GetArguments()["topic_key"].(string); ok {
			update.TopicKey = &v
		}
		if v, ok := req.GetArguments()["tags"].(string); ok {
			tags := parseTags(v)
			update.Tags = &tags
		}

		if update.Title == nil && update.Content == nil && update.Type == nil && update.Project == nil && update.Scope == nil && update.TopicKey == nil && update.Tags == nil {
			return mcp.NewToolResultError("provide at least one field to update"), nil
		}

		var contentLen int
		if update.Content != nil {
			contentLen = len(*update.Content)
		}

		obs, err := s.UpdateObservation(id, update)
		if err != nil {
			return mcp.NewToolResultError("Failed to update memory: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Memory updated: #%d %q (%s, scope=%s)", obs.ID, obs.Title, obs.Type, obs.Scope)
		if contentLen > s.MaxObservationLength() {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", contentLen, s.MaxObservationLength())
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleDelete(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		hardDelete := boolArg(req, "hard_delete", false)
		if err := s.DeleteObservation(id, hardDelete); err != nil {
			return mcp.NewToolResultError("Failed to delete memory: " + err.Error()), nil
		}

		mode := "soft-deleted"
		if hardDelete {
			mode = "permanently deleted"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Memory #%d %s", id, mode)), nil
	}
}

func handleSavePrompt(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		project, _ := req.GetArguments()["project"].(string)

		if sessionID == "" {
			sessionID = defaultSessionID(project)
		}

		if err := s.CreateSession(sessionID, project, ""); err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}

		_, err := s.AddPrompt(store.AddPromptParams{
			SessionID: sessionID,
			Content:   content,
			Project:   project,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save prompt: " + err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Prompt saved: %q", truncate(content, 80))), nil
	}
}

func handleContext(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		tagsRaw, _ := req.GetArguments()["tags"].(string)
		boostTagsRaw, _ := req.GetArguments()["prefer_tags"].(string)

		ctx2, err := s.FormatContextOpts(project, scope, store.ContextOptions{
			Tags:      parseTags(tagsRaw),
			PreferTags: parseTags(boostTagsRaw),
			TopicKey:  topicKey,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to get context: " + err.Error()), nil
		}

		if ctx2 == "" {
			return mcp.NewToolResultText("No previous session memories found."), nil
		}

		stats, _ := s.Stats()
		var projects string
		if len(stats.Projects) > 0 {
			projects = strings.Join(stats.Projects, ", ")
		} else {
			projects = "none"
		}

		result := fmt.Sprintf("%s\n---\nMemory stats: %d sessions, %d observations across projects: %s",
			ctx2, stats.TotalSessions, stats.TotalObservations, projects)

		return mcp.NewToolResultText(result), nil
	}
}

func handleListTags(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, _ := req.GetArguments()["project"].(string)

		tags, err := s.ListTags(project)
		if err != nil {
			return mcp.NewToolResultError("Failed to list tags: " + err.Error()), nil
		}

		if len(tags) == 0 {
			msg := "No tags found"
			if project != "" {
				msg += " for project " + project
			}
			return mcp.NewToolResultText(msg + "."), nil
		}

		var b strings.Builder
		if project != "" {
			fmt.Fprintf(&b, "Tags for project %q (%d total):\n\n", project, len(tags))
		} else {
			fmt.Fprintf(&b, "Tags across all projects (%d total):\n\n", len(tags))
		}
		for _, ti := range tags {
			fmt.Fprintf(&b, "  %-30s %d uses  (last: %s)\n", ti.Tag, ti.Count, ti.LastUsedAt)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

func handleTagStats(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, _ := req.GetArguments()["project"].(string)
		unusedSinceStr, _ := req.GetArguments()["unused_since"].(string)

		sortBy, _ := req.GetArguments()["sort_by"].(string)
		validSortKeys := map[string]bool{"": true, "freq": true, "stale": true, "alpha": true}
		if !validSortKeys[sortBy] {
			return mcp.NewToolResultError(fmt.Sprintf("invalid sort_by %q: accepted values are 'freq', 'stale', 'alpha'", sortBy)), nil
		}

		minCount := intArg(req, "min_count", 0)
		maxCount := intArg(req, "max_count", 0)
		limit := intArg(req, "limit", 20)
		if minCount < 0 {
			return mcp.NewToolResultError("min_count must be >= 0"), nil
		}
		if maxCount < 0 {
			return mcp.NewToolResultError("max_count must be >= 0"), nil
		}
		if limit < 0 {
			return mcp.NewToolResultError("limit must be >= 0"), nil
		}

		opts := store.TagStatsOptions{
			MinCount: minCount,
			MaxCount: maxCount,
			Limit:    limit,
			SortBy:   sortBy,
		}
		if unusedSinceStr != "" {
			t, err := time.Parse(time.RFC3339, unusedSinceStr)
			if err != nil {
				// Try date-only format as a convenience.
				t, err = time.Parse("2006-01-02", unusedSinceStr)
				if err != nil {
					return mcp.NewToolResultError("invalid unused_since format: use ISO8601 (e.g. '2026-01-01T00:00:00Z' or '2026-01-01')"), nil
				}
			}
			opts.UnusedSince = t
		}

		tags, err := s.TagStats(project, opts)
		if err != nil {
			return mcp.NewToolResultError("Failed to get tag stats: " + err.Error()), nil
		}

		if len(tags) == 0 {
			msg := "No tags match the given filters"
			if project != "" {
				msg += " for project " + project
			}
			return mcp.NewToolResultText(msg + "."), nil
		}

		var b strings.Builder
		label := "all projects"
		if project != "" {
			label = fmt.Sprintf("project %q", project)
		}
		fmt.Fprintf(&b, "Tag stats for %s (%d tags):\n\n", label, len(tags))
		fmt.Fprintf(&b, "  %-30s %6s   %s\n", "Tag", "Count", "Last used")
		fmt.Fprintf(&b, "  %-30s %6s   %s\n", strings.Repeat("-", 30), "------", "---------")
		for _, ti := range tags {
			fmt.Fprintf(&b, "  %-30s %6d   %s\n", ti.Tag, ti.Count, ti.LastUsedAt)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

func handleMergeTags(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		from, _ := req.GetArguments()["from"].(string)
		to, _ := req.GetArguments()["to"].(string)
		if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
			return mcp.NewToolResultError("both 'from' and 'to' are required"), nil
		}
		// Normalize to so the output message shows the canonical target form.
		canonicalTo := store.NormalizeTag(to)
		if canonicalTo == "" {
			return mcp.NewToolResultError(fmt.Sprintf("invalid target tag %q: empty after normalization", to)), nil
		}
		obsCount, sessCount, err := s.MergeTags(from, to)
		if err != nil {
			return mcp.NewToolResultError("merge failed: " + err.Error()), nil
		}
		msg := fmt.Sprintf("Merged %q → %q: %d observation(s), %d session(s) updated.", from, canonicalTo, obsCount, sessCount)
		return mcp.NewToolResultText(msg), nil
	}
}

func handleRelatedTags(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tag, _ := req.GetArguments()["tag"].(string)
		if strings.TrimSpace(tag) == "" {
			return mcp.NewToolResultError("'tag' is required"), nil
		}
		project, _ := req.GetArguments()["project"].(string)
		limit := intArg(req, "limit", 20)
		minCooc := intArg(req, "min_cooccurrence", 0)
		sinceStr, _ := req.GetArguments()["since"].(string)
		includeObs := boolArg(req, "include_observations", true)
		includeSes := boolArg(req, "include_sessions", true)

		if limit < 0 {
			return mcp.NewToolResultError("limit must be >= 0"), nil
		}

		opts := store.RelatedTagsOptions{
			Limit:               limit,
			MinCooccurrence:     minCooc,
			IncludeObservations: includeObs,
			IncludeSessions:     includeSes,
		}
		if sinceStr != "" {
			t, err := time.Parse(time.RFC3339, sinceStr)
			if err != nil {
				t, err = time.Parse("2006-01-02", sinceStr)
				if err != nil {
					return mcp.NewToolResultError("invalid since format: use ISO8601 (e.g. '2026-01-01T00:00:00Z' or '2026-01-01')"), nil
				}
			}
			opts.Since = t
		}

		related, err := s.RelatedTags(project, tag, opts)
		if err != nil {
			return mcp.NewToolResultError("Failed to get related tags: " + err.Error()), nil
		}

		if len(related) == 0 {
			msg := fmt.Sprintf("No tags co-occur with %q", tag)
			if project != "" {
				msg += " in project " + project
			}
			return mcp.NewToolResultText(msg + "."), nil
		}

		var b strings.Builder
		label := "all projects"
		if project != "" {
			label = fmt.Sprintf("project %q", project)
		}
		fmt.Fprintf(&b, "Tags related to %q in %s (%d results):\n\n", tag, label, len(related))
		fmt.Fprintf(&b, "  %-30s %12s   %s\n", "Tag", "Cooccurrences", "Last seen")
		fmt.Fprintf(&b, "  %-30s %12s   %s\n", strings.Repeat("-", 30), "-------------", "---------")
		for _, rt := range related {
			fmt.Fprintf(&b, "  %-30s %12d   %s\n", rt.Tag, rt.CooccurrenceCount, rt.LastSeenAt)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

func handleStats(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats, err := loadMCPStats(s)
		if err != nil {
			return mcp.NewToolResultError("Failed to get stats: " + err.Error()), nil
		}

		var projects string
		if len(stats.Projects) > 0 {
			projects = strings.Join(stats.Projects, ", ")
		} else {
			projects = "none yet"
		}

		result := fmt.Sprintf("Memory System Stats:\n- Sessions: %d\n- Observations: %d\n- Prompts: %d\n- Projects: %s",
			stats.TotalSessions, stats.TotalObservations, stats.TotalPrompts, projects)

		return mcp.NewToolResultText(result), nil
	}
}

func handleTimeline(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		observationID := int64(intArg(req, "observation_id", 0))
		if observationID == 0 {
			return mcp.NewToolResultError("observation_id is required"), nil
		}
		before := intArg(req, "before", 5)
		after := intArg(req, "after", 5)

		result, err := s.Timeline(observationID, before, after)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Timeline error: %s", err)), nil
		}

		var b strings.Builder

		if result.SessionInfo != nil {
			summary := ""
			if result.SessionInfo.Summary != nil {
				summary = fmt.Sprintf(" — %s", truncate(*result.SessionInfo.Summary, 100))
			}
			fmt.Fprintf(&b, "Session: %s (%s)%s\n", result.SessionInfo.Project, result.SessionInfo.StartedAt, summary)
			fmt.Fprintf(&b, "Total observations in session: %d\n\n", result.TotalInRange)
		}

		if len(result.Before) > 0 {
			b.WriteString("─── Before ───\n")
			for _, e := range result.Before {
				fmt.Fprintf(&b, "  #%d [%s] %s — %s\n", e.ID, e.Type, e.Title, truncate(e.Content, 150))
			}
			b.WriteString("\n")
		}

		fmt.Fprintf(&b, ">>> #%d [%s] %s <<<\n", result.Focus.ID, result.Focus.Type, result.Focus.Title)
		fmt.Fprintf(&b, "    %s\n", truncate(result.Focus.Content, 500))
		fmt.Fprintf(&b, "    %s\n\n", result.Focus.CreatedAt)

		if len(result.After) > 0 {
			b.WriteString("─── After ───\n")
			for _, e := range result.After {
				fmt.Fprintf(&b, "  #%d [%s] %s — %s\n", e.ID, e.Type, e.Title, truncate(e.Content, 150))
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

func handleGetObservation(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		obs, err := s.GetObservation(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Observation #%d not found", id)), nil
		}

		project := ""
		if obs.Project != nil {
			project = fmt.Sprintf("\nProject: %s", *obs.Project)
		}
		scope := fmt.Sprintf("\nScope: %s", obs.Scope)
		topic := ""
		if obs.TopicKey != nil {
			topic = fmt.Sprintf("\nTopic: %s", *obs.TopicKey)
		}
		toolName := ""
		if obs.ToolName != nil {
			toolName = fmt.Sprintf("\nTool: %s", *obs.ToolName)
		}
		duplicateMeta := fmt.Sprintf("\nDuplicates: %d", obs.DuplicateCount)
		revisionMeta := fmt.Sprintf("\nRevisions: %d", obs.RevisionCount)

		result := fmt.Sprintf("#%d [%s] %s\n%s\nSession: %s%s%s\nCreated: %s",
			obs.ID, obs.Type, obs.Title,
			obs.Content,
			obs.SessionID, project+scope+topic, toolName+duplicateMeta+revisionMeta,
			obs.CreatedAt,
		)

		return mcp.NewToolResultText(result), nil
	}
}

func handleSessionSummary(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		project, _ := req.GetArguments()["project"].(string)

		if sessionID == "" {
			sessionID = defaultSessionID(project)
		}

		if err := s.CreateSession(sessionID, project, ""); err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}

		_, err := s.AddObservation(store.AddObservationParams{
			SessionID: sessionID,
			Type:      "session_summary",
			Title:     fmt.Sprintf("Session summary: %s", project),
			Content:   content,
			Project:   project,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save session summary: " + err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Session summary saved for project %q", project)), nil
	}
}

func handleSessionStart(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := req.GetArguments()["id"].(string)
		project, _ := req.GetArguments()["project"].(string)
		directory, _ := req.GetArguments()["directory"].(string)

		if err := s.CreateSession(id, project, directory); err != nil {
			return mcp.NewToolResultError("Failed to start session: " + err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Session %q started for project %q", id, project)), nil
	}
}

func handleSessionEnd(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := req.GetArguments()["id"].(string)
		summary, _ := req.GetArguments()["summary"].(string)
		tagsRaw, _ := req.GetArguments()["tags"].(string)

		if err := s.EndSession(id, summary); err != nil {
			return mcp.NewToolResultError("Failed to end session: " + err.Error()), nil
		}

		if tags := parseTags(tagsRaw); len(tags) > 0 {
			if err := s.SetSessionTags(id, tags); err != nil {
				return mcp.NewToolResultError("Failed to set session tags: " + err.Error()), nil
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("Session %q completed", id)), nil
	}
}

func handleCapturePassive(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		project, _ := req.GetArguments()["project"].(string)
		source, _ := req.GetArguments()["source"].(string)

		if content == "" {
			return mcp.NewToolResultError("content is required — include text with a '## Key Learnings:' section"), nil
		}

		if sessionID == "" {
			sessionID = defaultSessionID(project)
			_ = s.CreateSession(sessionID, project, "")
		}

		if source == "" {
			source = "mcp-passive"
		}

		result, err := s.PassiveCapture(store.PassiveCaptureParams{
			SessionID: sessionID,
			Content:   content,
			Project:   project,
			Source:    source,
		})
		if err != nil {
			return mcp.NewToolResultError("Passive capture failed: " + err.Error()), nil
		}

		msg := fmt.Sprintf(
			"Passive capture complete: extracted=%d saved=%d duplicates=%d",
			result.Extracted, result.Saved, result.Duplicates,
		)
		if len(result.SuggestedTags) > 0 {
			msg += fmt.Sprintf("\nSuggested tags: %s", strings.Join(result.SuggestedTags, ", "))
		}
		return mcp.NewToolResultText(msg), nil
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func defaultSessionID(project string) string {
	if project == "" {
		return "manual-save"
	}
	return "manual-save-" + project
}

func intArg(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

func boolArg(req mcp.CallToolRequest, key string, defaultVal bool) bool {
	v, ok := req.GetArguments()[key].(bool)
	if !ok {
		return defaultVal
	}
	return v
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
