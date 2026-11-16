package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmeiracorbal/mnemo/v2/protocol"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var loadMCPStats = func(ctx context.Context, s MemoryBackend) (protocol.StatsOutput, error) {
	return s.Stats(ctx)
}

func handleStats(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats, err := loadMCPStats(ctx, s)
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

func handleTimeline(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, _ := req.GetArguments()["ref"].(string)
		ref, err := protocol.ParseResourceRef(raw)
		if err != nil { return mcp.NewToolResultError("Invalid Mnemo reference: " + err.Error()), nil }
		timeline, err := s.Timeline(ctx, protocol.ObservationTimelineInput{
			Ref: ref, Before: intArg(req, "before", 5), After: intArg(req, "after", 5),
		})
		if err != nil { return mcp.NewToolResultError("Timeline error: " + err.Error()), nil }
		var b strings.Builder
		fmt.Fprintf(&b, "Session: %s (%s)\n", timeline.SessionID, timeline.SessionStart)
		fmt.Fprintf(&b, "Total observations in session: %d\n\n", timeline.TotalInRange)
		if len(timeline.Before) > 0 {
			b.WriteString("--- Before ---\n")
			for _, o := range timeline.Before {
				fmt.Fprintf(&b, "%s [%s] %s — %s\n", o.Ref.String(), o.Type, o.Title, truncate(o.Content, 150))
			}
			b.WriteString("\n")
		}
		focus := timeline.Focus
		fmt.Fprintf(&b, ">>> %s [%s] %s <<<\n", focus.Ref.String(), focus.Type, focus.Title)
		fmt.Fprintf(&b, "    %s\n    %s\n\n", truncate(focus.Content, 500), focus.CreatedAt)
		if len(timeline.After) > 0 {
			b.WriteString("--- After ---\n")
			for _, o := range timeline.After {
				fmt.Fprintf(&b, "%s [%s] %s — %s\n", o.Ref.String(), o.Type, o.Title, truncate(o.Content, 150))
			}
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}
