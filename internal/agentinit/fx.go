package agentinit

import (
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

func fxLabel() string { return "fx" }

func fxDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".fx")}
}

func fxInstructionPath(home string) string {
	return filepath.Join(home, ".fx", "AGENTS.md")
}

func fxInstallInstructions(home string) (string, error) {
	path := fxInstructionPath(home)
	if err := AppendSection(path, templates.Global+"\n\n"+templates.Fx); err != nil {
		return "", err
	}
	return path, nil
}

func fxRemoveInstructions(home string) (string, bool, error) {
	path := fxInstructionPath(home)
	changed, err := RemoveSection(path)
	return path, changed, err
}

func fxConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	return []ConfigSnippet{{
		Agent:  fxLabel(),
		Path:   filepath.Join(home, ".fx", "mcp.json"),
		Format: "json",
		Content: prettyJSON(map[string]any{
			"mcp": map[string]any{
				"mnemo": map[string]any{
					"command": []string{mnemoBin, "mcp", "--tools=agent"},
					"environment": map[string]string{
						"MNEMO_AGENT":         AgentFx,
						"MNEMO_MCP_CLIENT":    AgentFx,
						"MNEMO_MCP_TRANSPORT": "stdio",
					},
				},
			},
		}),
	}}
}

func fxRuntimeAssets() []assetTarget {
	return nil
}

func fxUninstallConfig(home string) ([]string, error) {
	path := filepath.Join(home, ".fx", "mcp.json")
	changed, err := removeMCPServer(path, "mcp", "mnemo")
	return appendChanged(path, changed, err)
}

func fxCheckInstructions(home string) Check {
	return checkInstructionFile("fx", fxInstructionPath(home), true)
}

func fxCheckMCP(home string) Check {
	path := filepath.Join(home, ".fx", "mcp.json")
	return checkJSONMCPWithEnv(path, "mcp_config.fx", "fx", "environment", "mcp", "mnemo")
}

func fxCheckRuntime(home string) Check {
	return Check{}
}
