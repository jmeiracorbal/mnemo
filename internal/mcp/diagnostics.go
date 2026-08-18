package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	"github.com/jmeiracorbal/mnemo/internal/doctor"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDiagnosticTools(srv *server.MCPServer, allowlist map[string]bool) {
	if shouldRegister("mem_current_project", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_current_project",
				mcp.WithDescription(strings.TrimSpace(memCurrentProjectDescription)),
				mcp.WithTitleAnnotation("Current Project"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("path",
					mcp.Description("Workspace path to resolve (default: current working directory)"),
				),
			),
			handleCurrentProject(),
		)
	}

	if shouldRegister("mem_doctor", allowlist) {
		srv.AddTool(
			mcp.NewTool("mem_doctor",
				mcp.WithDescription(strings.TrimSpace(memDoctorDescription)),
				mcp.WithTitleAnnotation("Integration Doctor"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(true),
				mcp.WithOpenWorldHintAnnotation(false),
				mcp.WithString("path",
					mcp.Description("Project path to diagnose (default: current working directory)"),
				),
				mcp.WithString("agent",
					mcp.Description("Agent integration to check: claudecode, cursor, windsurf, codex, opencode, or all (default: all)"),
				),
			),
			handleDoctor(),
		)
	}
}

type currentProjectResult struct {
	Status      string   `json:"status"`
	ProjectRoot string   `json:"project_root"`
	ProjectID   string   `json:"project_id,omitempty"`
	Agents      []string `json:"agents,omitempty"`
	MarkerPath  string   `json:"marker_path"`
	Message     string   `json:"message,omitempty"`
}

func handleCurrentProject() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, _ := req.GetArguments()["path"].(string)
		if strings.TrimSpace(path) == "" {
			path = "."
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			return mcp.NewToolResultError("resolve path: " + err.Error()), nil
		}

		root := agentinit.ProjectRoot(abs)
		markerPath := filepath.Join(root, ".mnemo")
		result := currentProjectResult{
			ProjectRoot: root,
			MarkerPath:  markerPath,
		}

		projectID, err := agentinit.ReadProjectID(root)
		if err != nil {
			result.Status = "inactive"
			result.Message = err.Error()
			out, marshalErr := json.MarshalIndent(result, "", "  ")
			if marshalErr != nil {
				return nil, fmt.Errorf("marshal current project result: %w", marshalErr)
			}
			return mcp.NewToolResultText(string(out)), nil
		}

		marker, err := agentinit.ReadProjectMarker(root)
		if err != nil {
			return mcp.NewToolResultError("read project marker: " + err.Error()), nil
		}

		result.Status = "active"
		result.ProjectID = projectID
		result.Agents = append([]string(nil), marker.Agents...)
		result.Message = "mnemo is active for this project; use this project ID in memory tool calls"

		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal current project result: %w", err)
		}
		return mcp.NewToolResultText(string(out)), nil
	}
}

func handleDoctor() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, _ := req.GetArguments()["path"].(string)
		if strings.TrimSpace(path) == "" {
			path = "."
		}
		agent, _ := req.GetArguments()["agent"].(string)
		if strings.TrimSpace(agent) == "" {
			agent = "all"
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return mcp.NewToolResultError("resolve home directory: " + err.Error()), nil
		}

		report := doctor.BuildReport(doctor.Options{
			Agent: agent,
			Path:  path,
			Home:  home,
		})

		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal doctor report: %w", err)
		}
		return mcp.NewToolResultText(string(out)), nil
	}
}
