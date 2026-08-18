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
	return []ConfigSnippet{{
		Agent:  openCodeLabel(),
		Path:   filepath.Join(home, ".config", "opencode", "opencode.json"),
		Format: "json",
		Content: prettyJSON(map[string]any{
			"mcp": map[string]any{
				"mnemo": map[string]any{
					"type":    "local",
					"command": []string{mnemoBin, "mcp", "--tools=agent"},
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
	changed, err := removeMCPServer(path, "mcp", "mnemo")
	return appendChanged(path, changed, err)
}

func openCodeCheckInstructions(home string) Check {
	return checkInstructionFile("opencode", openCodeInstructionPath(home), true)
}

func openCodeCheckMCP(home string) Check {
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	return checkJSONHas(path, "mcp_config.opencode", "opencode", "mcp", "mnemo")
}

func openCodeCheckRuntime(home string) Check {
	paths := []string{
		filepath.Join(home, ".config", "opencode", "plugins", "mnemo.ts"),
		filepath.Join(home, ".config", "opencode", "plugins", "mnemo-protocol.md"),
	}
	return checkFiles("opencode", "runtime_files.opencode", "OpenCode plugin files installed", paths, false)
}
