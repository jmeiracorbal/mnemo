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
	// Pi's native extension carries ctx.sessionManager.getSessionId() on every
	// call. A static MCP child cannot receive that dynamic identity, so Pi owns
	// no mnemo MCP configuration.
	return nil
}

func piRuntimeAssets() []assetTarget {
	return []assetTarget{{Asset: "scripts/pi/extensions/mnemo.ts", Path: filepath.Join(".pi", "agent", "extensions", "mnemo.ts"), Mode: 0644}}
}

func piCheckRuntime(home string) Check {
	return checkFiles("pi", "runtime_files.pi", "Pi lifecycle extension installed", []string{filepath.Join(home, ".pi", "agent", "extensions", "mnemo.ts")}, false)
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
	check := piCheckRuntime(home)
	check.ID = "mcp_config.pi"
	if check.Status == "ok" {
		check.Message = "Pi native extension owns mnemo tool transport"
	}
	return check
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
