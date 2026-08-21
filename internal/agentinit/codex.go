package agentinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/templates"
)

func codexLabel() string { return "Codex" }

func codexDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".codex")}
}

func codexInstructionPath(home string) string {
	return filepath.Join(home, ".codex", "AGENTS.md")
}

func codexInstallInstructions(home string) (string, error) {
	path := codexInstructionPath(home)
	if err := AppendSection(path, templates.Global); err != nil {
		return "", err
	}
	return path, nil
}

func codexRemoveInstructions(home string) (string, bool, error) {
	path := codexInstructionPath(home)
	changed, err := RemoveSection(path)
	return path, changed, err
}

func codexConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	hooksDir := filepath.Join(home, ".codex", "hooks")
	return []ConfigSnippet{
		{
			Agent:   codexLabel(),
			Path:    filepath.Join(home, ".codex", "config.toml"),
			Format:  "toml",
			Content: fmt.Sprintf("[mcp_servers.mnemo]\ncommand = %q\nargs = [\"mcp\", \"--tools=agent\"]\n\n[mcp_servers.mnemo.env]\nMNEMO_AGENT = %q\nMNEMO_MCP_CLIENT = %q\nMNEMO_MCP_TRANSPORT = \"stdio\"\n", mnemoBin, AgentCodex, AgentCodex),
		},
		{
			Agent:  codexLabel(),
			Path:   filepath.Join(home, ".codex", "hooks.json"),
			Format: "json",
			Content: prettyJSON(map[string]any{
				"hooks": map[string]any{
					"SessionStart": []any{map[string]any{
						"matcher": "startup|resume",
						"hooks": []any{map[string]any{
							"type":          "command",
							"command":       filepath.Join(hooksDir, "session-start.sh"),
							"statusMessage": "Loading mnemo memory...",
							"timeout":       10,
						}},
					}},
					"Stop": []any{map[string]any{
						"matcher": "",
						"hooks": []any{map[string]any{
							"type":    "command",
							"command": filepath.Join(hooksDir, "stop.sh"),
							"timeout": 10,
						}},
					}},
				},
			}),
		},
	}
}

func codexRuntimeAssets() []assetTarget {
	return []assetTarget{
		{Asset: "scripts/codex/hooks/session-start.sh", Path: filepath.Join(".codex", "hooks", "session-start.sh"), Mode: 0755},
		{Asset: "scripts/codex/hooks/stop.sh", Path: filepath.Join(".codex", "hooks", "stop.sh"), Mode: 0755},
		{Asset: "scripts/codex/hooks/mnemo-protocol.md", Path: filepath.Join(".codex", "hooks", "mnemo-protocol.md"), Mode: 0644},
	}
}

func codexUninstallConfig(home string) ([]string, error) {
	var removed []string
	configPath := filepath.Join(home, ".codex", "config.toml")
	changed, err := removeCodexMCPConfig(configPath)
	paths, err := appendChanged(configPath, changed, err)
	removed = append(removed, paths...)
	if err != nil {
		return removed, err
	}

	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	hooksDir := filepath.Join(home, ".codex", "hooks")
	changed, err = removeHookCommands(hooksPath, map[string]string{
		"SessionStart": filepath.Join(hooksDir, "session-start.sh"),
		"Stop":         filepath.Join(hooksDir, "stop.sh"),
	})
	paths, err = appendChanged(hooksPath, changed, err)
	removed = append(removed, paths...)
	return removed, err
}

func codexCheckInstructions(home string) Check {
	return checkInstructionFile("codex", codexInstructionPath(home), true)
}

func codexCheckMCP(home string) Check {
	path := filepath.Join(home, ".codex", "config.toml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return checkWarning("codex", "mcp_config.codex", "MCP config not found", path)
	}
	if err != nil {
		return checkError("codex", "mcp_config.codex", "read MCP config: "+err.Error(), path)
	}
	content := string(data)
	if !strings.Contains(content, "[mcp_servers.mnemo]") || !strings.Contains(content, "mcp") {
		return checkWarning("codex", "mcp_config.codex", "MCP config does not contain mnemo server", path)
	}
	if !strings.Contains(content, "[mcp_servers.mnemo.env]") ||
		!strings.Contains(content, `MNEMO_AGENT = "codex"`) {
		return checkWarning("codex", "mcp_config.codex", "MCP config is missing mnemo provenance environment", path)
	}
	return checkOK("codex", "mcp_config.codex", "MCP config contains mnemo server", path)
}

func codexCheckRuntime(home string) Check {
	paths := []string{
		filepath.Join(home, ".codex", "hooks.json"),
		filepath.Join(home, ".codex", "hooks", "session-start.sh"),
		filepath.Join(home, ".codex", "hooks", "stop.sh"),
		filepath.Join(home, ".codex", "hooks", "mnemo-protocol.md"),
	}
	return checkFiles("codex", "runtime_files.codex", "Codex global hooks installed", paths, true)
}

func upsertCodexMCPConfig(path, section string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	var out []string
	skipping := false
	replaced := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCodexMnemoTableHeader(trimmed) {
			if !replaced {
				out = append(out, strings.TrimRight(section, "\n"))
			}
			skipping = true
			replaced = true
			continue
		}
		if skipping && isTOMLTableHeader(trimmed) {
			skipping = false
		}
		if !skipping {
			out = append(out, line)
		}
	}
	content := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if !replaced {
		content = strings.TrimRight(string(data), "\n")
		if content != "" {
			content += "\n\n"
		}
		content += strings.TrimRight(section, "\n")
	}
	content += "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func removeCodexMCPConfig(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	removed := false
	skipping := false
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCodexMnemoTableHeader(trimmed) {
			removed = true
			skipping = true
			continue
		}
		if skipping && isTOMLTableHeader(trimmed) {
			skipping = false
		}
		if !skipping {
			out = append(out, line)
		}
	}
	if !removed {
		return false, nil
	}

	content := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	content += "\n"
	return true, os.WriteFile(path, []byte(content), 0644)
}

func isCodexMnemoTableHeader(line string) bool {
	name, ok := tomlTableName(line)
	return ok && (name == "mcp_servers.mnemo" || strings.HasPrefix(name, "mcp_servers.mnemo."))
}

func isTOMLTableHeader(line string) bool {
	_, ok := tomlTableName(line)
	return ok
}

func tomlTableName(line string) (string, bool) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "[[") && strings.HasSuffix(line, "]]"):
		return strings.TrimSpace(line[2 : len(line)-2]), true
	case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
		return strings.TrimSpace(line[1 : len(line)-1]), true
	default:
		return "", false
	}
}
