package agentinit

import (
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

func openCodeLabel() string { return "OpenCode" }

func openCodeDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".config", "opencode")}
}

func openCodeInstructionPath(home string) string {
	return filepath.Join(home, ".config", "opencode", "AGENTS.md")
}

func openCodeInstallInstructions(home string) (string, error) {
	path := openCodeInstructionPath(home)
	if err := AppendSection(path, templates.Global); err != nil {
		return "", err
	}
	return path, nil
}

func openCodeRemoveInstructions(home string) (string, bool, error) {
	path := openCodeInstructionPath(home)
	changed, err := RemoveSection(path)
	return path, changed, err
}

func openCodeConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	server := map[string]any{
		"type":    "local",
		"command": []string{mnemoBin, "mcp", "--tools=agent"},
		"environment": map[string]string{
			"MNEMO_AGENT":         AgentOpenCode,
			"MNEMO_MCP_CLIENT":    AgentOpenCode,
			"MNEMO_MCP_TRANSPORT": "stdio",
		},
	}
	return []ConfigSnippet{{
		Agent:  openCodeLabel(),
		Path:   filepath.Join(home, ".config", "opencode", "opencode.json"),
		Format: "json",
		Content: prettyJSON(map[string]any{
			"mcp": map[string]any{
				"servers": map[string]any{
					"mnemo": server,
				},
			},
		}),
	}}
}

func openCodeRuntimeAssets() []assetTarget {
	return []assetTarget{
		{Asset: "scripts/opencode/plugins/mnemo.ts", Path: filepath.Join(".config", "opencode", "plugins", "mnemo.ts"), Mode: 0644},
		{Asset: "scripts/opencode/plugins/mnemo-protocol.md", Path: filepath.Join(".config", "opencode", "plugins", "mnemo-protocol.md"), Mode: 0644},
	}
}

func openCodeUninstallConfig(home string) ([]string, error) {
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	var removed []string
	changed, err := removeMCPServer(path, "mcp", "mnemo")
	if err != nil {
		return removed, err
	}
	if changed {
		removed = append(removed, path)
	}
	changed, err = removeNestedMCPServer(path, "mcp", "servers", "mnemo")
	if err != nil {
		return removed, err
	}
	if changed && len(removed) == 0 {
		removed = append(removed, path)
	}
	return removed, nil
}

func openCodeCheckInstructions(home string) Check {
	return checkInstructionFile("opencode", openCodeInstructionPath(home), true)
}

func openCodeCheckMCP(home string) Check {
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	return checkJSONMCPWithEnv(path, "mcp_config.opencode", "opencode", "environment", "mcp", "servers", "mnemo")
}

func openCodeCheckRuntime(home string) Check {
	paths := []string{
		filepath.Join(home, ".config", "opencode", "plugins", "mnemo.ts"),
		filepath.Join(home, ".config", "opencode", "plugins", "mnemo-protocol.md"),
	}
	return checkFiles("opencode", "runtime_files.opencode", "OpenCode plugin files installed", paths, false)
}
