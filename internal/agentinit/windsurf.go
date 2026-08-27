package agentinit

import (
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

func windsurfLabel() string { return "Windsurf" }

func windsurfDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".codeium", "windsurf")}
}

func windsurfInstructionPath(home string) string {
	return filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md")
}

func windsurfSkillLinkPath(home string) string {
	return filepath.Join(home, ".codeium", "windsurf", "skills", globalSkillName)
}

func windsurfInstallInstructions(home string) (string, error) {
	path := windsurfInstructionPath(home)
	if err := AppendSection(path, templates.Global); err != nil {
		return "", err
	}
	return path, nil
}

func windsurfRemoveInstructions(home string) (string, bool, error) {
	path := windsurfInstructionPath(home)
	changed, err := RemoveSection(path)
	return path, changed, err
}

func windsurfConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
	return []ConfigSnippet{
		{
			Agent:   windsurfLabel(),
			Path:    filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"),
			Format:  "json",
			Content: mcpServersJSON(mnemoBin, AgentWindsurf),
		},
		{
			Agent:  windsurfLabel(),
			Path:   filepath.Join(home, ".codeium", "windsurf", "hooks.json"),
			Format: "json",
			Content: prettyJSON(map[string]any{
				"hooks": map[string]any{
					"pre_user_prompt":                       []any{map[string]any{"command": filepath.Join(hooksDir, "pre-user-prompt.sh")}},
					"post_cascade_response_with_transcript": []any{map[string]any{"command": filepath.Join(hooksDir, "post-cascade-response.sh")}},
				},
			}),
		},
	}
}

func windsurfRuntimeAssets() []assetTarget {
	return []assetTarget{
		{Asset: "scripts/windsurf/hooks/pre-user-prompt.sh", Path: filepath.Join(".codeium", "windsurf", "hooks", "pre-user-prompt.sh"), Mode: 0755},
		{Asset: "scripts/windsurf/hooks/post-cascade-response.sh", Path: filepath.Join(".codeium", "windsurf", "hooks", "post-cascade-response.sh"), Mode: 0755},
		{Asset: "assets/protocol/session-start-protocol.md", Path: filepath.Join(".codeium", "windsurf", "hooks", "session-start-protocol.md"), Mode: 0644},
	}
}

func windsurfUninstallConfig(home string) ([]string, error) {
	var removed []string
	mcpPath := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
	changed, err := removeMCPServer(mcpPath, "mcpServers", "mnemo")
	paths, err := appendChanged(mcpPath, changed, err)
	removed = append(removed, paths...)
	if err != nil {
		return removed, err
	}

	hooksPath := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
	changed, err = removeHookCommands(hooksPath, map[string]string{
		"pre_user_prompt":                       filepath.Join(hooksDir, "pre-user-prompt.sh"),
		"post_cascade_response_with_transcript": filepath.Join(hooksDir, "post-cascade-response.sh"),
	})
	paths, err = appendChanged(hooksPath, changed, err)
	removed = append(removed, paths...)
	return removed, err
}

func windsurfCheckInstructions(home string) Check {
	return checkInstructionFile("windsurf", windsurfInstructionPath(home), true)
}

func windsurfCheckMCP(home string) Check {
	path := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
	return checkJSONMCPWithEnv(path, "mcp_config.windsurf", "windsurf", "env", "mcpServers", "mnemo")
}

func windsurfCheckRuntime(home string) Check {
	hooksPath := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
	paths := []string{
		hooksPath,
		filepath.Join(hooksDir, "pre-user-prompt.sh"),
		filepath.Join(hooksDir, "post-cascade-response.sh"),
		filepath.Join(hooksDir, "session-start-protocol.md"),
	}
	check := checkFiles("windsurf", "runtime_files.windsurf", "Windsurf global hooks installed", paths, true)
	if check.Status != "ok" {
		return check
	}
	return checkJSONHookCommands(hooksPath, "runtime_files.windsurf", "windsurf", "Windsurf global hooks installed", map[string][]string{
		"pre_user_prompt":                       {filepath.Join(hooksDir, "pre-user-prompt.sh")},
		"post_cascade_response_with_transcript": {filepath.Join(hooksDir, "post-cascade-response.sh")},
	})
}
