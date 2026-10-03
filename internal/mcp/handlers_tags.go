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

func handleListTags(s MemoryBackend) server.ToolHandlerFunc {
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

func handleTagStats(s MemoryBackend) server.ToolHandlerFunc {
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

func handleMergeTags(s MemoryBackend) server.ToolHandlerFunc {
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

func handleRelatedTags(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tag, _ := req.GetArguments()["tag"].(string)
		if strings.TrimSpace(tag) == "" {
			return mcp.NewToolResultError("'tag' is required"), nil
		}
		project, _ := req.GetArguments()["project"].(string)
		limit := intArg(req, "limit", 20)
		minCooc := intArg(req, "min_cooccurrence", 0)
		sinceStr, _ := req.GetArguments()["since"].(string)
		includeObs, obsProvided := req.GetArguments()["include_observations"].(bool)
		if !obsProvided {
			includeObs = true
		}
		includeSes, sesProvided := req.GetArguments()["include_sessions"].(bool)
		if !sesProvided {
			includeSes = true
		}

		if limit < 0 {
			return mcp.NewToolResultError("limit must be >= 0"), nil
		}
		if minCooc < 0 {
			return mcp.NewToolResultError("min_cooccurrence must be >= 0"), nil
		}
		if obsProvided && sesProvided && !includeObs && !includeSes {
			return mcp.NewToolResultError("at least one of include_observations or include_sessions must be true"), nil
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
