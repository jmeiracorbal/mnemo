package agentinit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/templates"
)

var (
	errClaudePluginRegistryNotFound = errors.New("claude code plugin registry not found")
	errClaudeMnemoPluginNotFound    = errors.New("claude code mnemo plugin not found")
)

func claudeCodeLabel() string { return "Claude" }

func claudeCodeDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".claude")}
}

func claudeCodeInstructionPath(home string) string {
	return filepath.Join(home, ".claude", "CLAUDE.md")
}

func claudeCodeInstallInstructions(home string) (string, error) {
	path := claudeCodeInstructionPath(home)
	if err := AppendSection(path, templates.Global); err != nil {
		return "", err
	}
	return path, nil
}

func claudeCodeRemoveInstructions(home string) (string, bool, error) {
	path := claudeCodeInstructionPath(home)
	changed, err := RemoveSection(path)
	return path, changed, err
}

func claudeCodeConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	return []ConfigSnippet{{
		Agent:   claudeCodeLabel(),
		Path:    filepath.Join(home, ".claude", ".mcp.json"),
		Format:  "json",
		Content: mcpServersJSON(mnemoBin),
	}}
}

func claudeCodeRuntimeAssets() []assetTarget {
	// Claude Code hooks are managed by the Claude plugin installation.
	return nil
}

func claudeCodeUninstallConfig(home string) ([]string, error) {
	path := filepath.Join(home, ".claude", ".mcp.json")
	changed, err := removeMCPServer(path, "mcpServers", "mnemo")
	return appendChanged(path, changed, err)
}

func claudeCodeCheckInstructions(home string) Check {
	return checkInstructionFile("claudecode", claudeCodeInstructionPath(home), true)
}

func claudeCodeCheckMCP(home string) Check {
	path := filepath.Join(home, ".claude", ".mcp.json")
	return checkJSONHas(path, "mcp_config.claudecode", "claudecode", "mcpServers", "mnemo")
}

func claudeCodeCheckRuntime(home string) Check {
	installPath, err := claudeMnemoPluginInstallPath(home)
	if err != nil {
		// install.sh/setup refresh configure Claude Code through MCP and global
		// instructions. The Claude plugin is optional in that path, so an absent
		// plugin registry or mnemo plugin is not a broken runtime surface.
		if errors.Is(err, errClaudePluginRegistryNotFound) || errors.Is(err, errClaudeMnemoPluginNotFound) {
			return Check{}
		}
		path := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
		return checkError("claudecode", "runtime_files.claudecode", err.Error(), path)
	}
	paths := []string{
		filepath.Join(installPath, ".claude-plugin", "plugin.json"),
		filepath.Join(installPath, "hooks", "hooks.json"),
		filepath.Join(installPath, "scripts", "session-start.sh"),
		filepath.Join(installPath, "scripts", "session-stop.sh"),
		filepath.Join(installPath, "scripts", "subagent-stop.sh"),
		filepath.Join(installPath, "scripts", "post-compact.sh"),
		filepath.Join(installPath, "scripts", "post-compact-resume.sh"),
		filepath.Join(installPath, "scripts", "post-file-edit.sh"),
		filepath.Join(installPath, "scripts", "post-bash-git.sh"),
	}
	return checkFiles("claudecode", "runtime_files.claudecode", "Claude Code plugin hooks installed", paths, true)
}

func claudeMnemoPluginInstallPath(home string) (string, error) {
	path := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", errClaudePluginRegistryNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Claude Code plugin registry: %w", err)
	}
	var registry struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return "", fmt.Errorf("parse Claude Code plugin registry: %w", err)
	}
	raw, ok := registry.Plugins["mnemo@mnemo"]
	if !ok {
		return "", errClaudeMnemoPluginNotFound
	}
	var entries []struct {
		InstallPath string `json:"installPath"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		var entry struct {
			InstallPath string `json:"installPath"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return "", fmt.Errorf("parse Claude Code mnemo plugin entry: %w", err)
		}
		entries = []struct {
			InstallPath string `json:"installPath"`
		}{entry}
	}
	if len(entries) == 0 || strings.TrimSpace(entries[0].InstallPath) == "" {
		return "", errClaudeMnemoPluginNotFound
	}
	return entries[0].InstallPath, nil
}
