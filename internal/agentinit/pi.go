package agentinit

import (
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

func piLabel() string { return "Pi" }

func piDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".pi", "agent")}
}

func piInstructionPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "APPEND_SYSTEM.md")
}

func piSkillLinkPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "skills", globalSkillName)
}

func piInstallInstructions(home string) (string, error) {
	path := piInstructionPath(home)
	if err := AppendSection(path, templates.Global+"\n\n"+templates.Pi); err != nil {
		return "", err
	}
	return path, nil
}

func piRemoveInstructions(home string) (string, bool, error) {
	path := piInstructionPath(home)
	changed, err := RemoveSection(path)
	return path, changed, err
}

func piConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	return []ConfigSnippet{{
		Agent:   piLabel(),
		Path:    filepath.Join(home, ".pi", "agent", "mcp.json"),
		Format:  "json",
		Content: mcpServersJSON(mnemoBin, AgentPi),
	}}
}

func piUninstallConfig(home string) ([]string, error) {
	path := filepath.Join(home, ".pi", "agent", "mcp.json")
	changed, err := removeMCPServer(path, "mcpServers", "mnemo")
	return appendChanged(path, changed, err)
}

func piCheckInstructions(home string) Check {
	return checkInstructionFile("pi", piInstructionPath(home), true)
}

func piCheckMCP(home string) Check {
	path := filepath.Join(home, ".pi", "agent", "mcp.json")
	return checkJSONMCPWithEnv(path, "mcp_config.pi", "pi", "env", "mcpServers", "mnemo")
}

func piProjectInstructionPath(root string) string {
	return filepath.Join(root, ".pi", "APPEND_SYSTEM.md")
}

func piInstallProjectInstructions(root string) (string, error) {
	path := piProjectInstructionPath(root)
	if err := AppendSection(path, templates.Pi); err != nil {
		return "", err
	}
	return path, nil
}
