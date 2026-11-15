package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo/protocol"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func handleSessionSummary(s MemoryBackend, runtime *Runtime) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		project, _ := req.GetArguments()["project"].(string)
		directory, _ := req.GetArguments()["directory"].(string)

		if err := validateMarker(project, directory); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sessionID, err := runtime.resolveSession(s, project, directory, req)
		if err != nil {
			return mcp.NewToolResultError("No active session: " + err.Error()), nil
		}
		provenance := store.MCPProvenance(store.ToolMemSessionSummary)

		_, err = s.CreateObservation(ctx, uuid.NewString(), protocol.ObservationCreateInput{
			SessionID: sessionID, ProjectID: project, Type: "session_summary",
			Title: fmt.Sprintf("Session summary: %s", project),
			Content: content,
			Provenance: protocol.Provenance{
			AgentID: provenance.AgentID, SourceKindID: provenance.SourceKindID,
			ToolID: provenance.ToolID, ModelID: provenance.ModelID,
			ModelProvider: provenance.ModelProvider,
			MCPClientID: provenance.MCPClientID, MCPClientName: provenance.MCPClientName,
			MCPClientVersion: provenance.MCPClientVersion, MCPTransport: provenance.MCPTransport,
		},
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save session summary: " + err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Session summary saved for project %q", project)), nil
	}
}

func handleCapturePassive(s MemoryBackend, runtime *Runtime) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		project, _ := req.GetArguments()["project"].(string)
		directory, _ := req.GetArguments()["directory"].(string)
		source, _ := req.GetArguments()["source"].(string)

		if content == "" {
			return mcp.NewToolResultError("content is required — include text with a '## Key Learnings:' section"), nil
		}

		if err := validateMarker(project, directory); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sessionID, err := runtime.resolveSession(s, project, directory, req)
		if err != nil {
			return mcp.NewToolResultError("No active session: " + err.Error()), nil
		}

		if source == "" {
			source = "mcp-passive"
		}

		result, err := s.PassiveCapture(store.PassiveCaptureParams{
			SessionID:  sessionID,
			Content:    content,
			Project:    project,
			Source:     source,
			Provenance: store.MCPProvenance(store.ToolMemCapturePassive),
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
