package agentinit

import (
	"encoding/json"
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
	if !tomlHasSection(content, "[mcp_servers.mnemo]") {
		return checkWarning("codex", "mcp_config.codex", "MCP config does not contain mnemo server", path)
	}
	if !tomlSectionHasKey(content, "[mcp_servers.mnemo]", "command") ||
		!tomlSectionHasAssignment(content, "[mcp_servers.mnemo]", "args", `["mcp", "--tools=agent"]`) {
		return checkWarning("codex", "mcp_config.codex", "MCP config is missing mnemo agent tool invocation", path)
	}
	if !tomlHasSection(content, "[mcp_servers.mnemo.env]") ||
		!tomlSectionHasAssignment(content, "[mcp_servers.mnemo.env]", "MNEMO_AGENT", `"codex"`) {
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
	check := checkFiles("codex", "runtime_files.codex", "Codex global hooks installed", paths, true)
	if check.Status != "ok" {
		return check
	}
	if trustCheck := codexCheckHookTrust(home); trustCheck.ID != "" {
		return trustCheck
	}
	return check
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	_, err = migrateCodexHookFeatureFlag(path)
	return err
}

func migrateCodexHookFeatureFlag(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	inFeatures := false
	hooksSeen := false
	codexHooksValue := ""
	changed := false

	flushFeatures := func() {
		if inFeatures && codexHooksValue != "" && !hooksSeen {
			out = append(out, "hooks = "+codexHooksValue)
			changed = true
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTOMLTableHeader(trimmed) {
			flushFeatures()
			name, _ := tomlTableName(trimmed)
			inFeatures = name == "features"
			hooksSeen = false
			codexHooksValue = ""
		}
		if inFeatures {
			key, value, ok := tomlAssignment(trimmed)
			if ok {
				switch key {
				case "codex_hooks":
					if codexHooksValue == "" {
						codexHooksValue = value
					}
					changed = true
					continue
				case "hooks":
					hooksSeen = true
				}
			}
		}
		out = append(out, line)
	}
	flushFeatures()
	if !changed {
		return false, nil
	}

	content := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	return true, os.WriteFile(path, []byte(content), 0644)
}

func tomlAssignment(line string) (key, value string, ok bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	return key, value, key != ""
}

func codexCheckHookTrust(home string) Check {
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	configPath := filepath.Join(home, ".codex", "config.toml")
	identities, missingCommands, err := codexMnemoHookIdentities(hooksPath, home)
	if err != nil {
		return checkError("codex", "runtime_files.codex", "parse Codex hooks: "+err.Error(), hooksPath)
	}
	if len(missingCommands) > 0 {
		return Check{
			ID:       "runtime_files.codex",
			Status:   "warning",
			Severity: "warning",
			Agent:    "codex",
			Message:  "Codex hooks.json is missing mnemo hook commands",
			Path:     hooksPath,
			Details:  map[string]string{"missing_commands": strings.Join(missingCommands, ",")},
		}
	}

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return Check{
			ID:       "runtime_files.codex",
			Status:   "warning",
			Severity: "warning",
			Agent:    "codex",
			Message:  "Codex mnemo hooks need review in Codex",
			Path:     hooksPath,
			Details:  map[string]string{"missing_trust": strings.Join(codexHookIdentityStrings(identities), ",")},
		}
	}
	if err != nil {
		return checkError("codex", "runtime_files.codex", "read Codex config: "+err.Error(), configPath)
	}

	var missingTrust []string
	content := string(data)
	for _, identity := range identities {
		if !tomlHasSection(content, codexHookTrustSection(identity.Identity)) {
			missingTrust = append(missingTrust, identity.String())
		}
	}
	if len(missingTrust) > 0 {
		return Check{
			ID:       "runtime_files.codex",
			Status:   "warning",
			Severity: "warning",
			Agent:    "codex",
			Message:  "Codex mnemo hooks need review in Codex",
			Path:     hooksPath,
			Details:  map[string]string{"missing_trust": strings.Join(missingTrust, ",")},
		}
	}
	return Check{}
}

type codexHookIdentity struct {
	Event    string
	Identity string
	Command  string
}

func (i codexHookIdentity) String() string {
	return i.Event + ":" + i.Command
}

func codexHookIdentityStrings(identities []codexHookIdentity) []string {
	out := make([]string, 0, len(identities))
	for _, identity := range identities {
		out = append(out, identity.String())
	}
	return out
}

func codexMnemoHookIdentities(hooksPath, home string) ([]codexHookIdentity, []string, error) {
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		return nil, nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, nil, err
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return nil, []string{"SessionStart", "Stop"}, nil
	}

	expectedCommands := map[string]string{
		"SessionStart": filepath.Join(home, ".codex", "hooks", "session-start.sh"),
		"Stop":         filepath.Join(home, ".codex", "hooks", "stop.sh"),
	}
	events := []string{"SessionStart", "Stop"}
	var identities []codexHookIdentity
	var missing []string
	for _, event := range events {
		command := expectedCommands[event]
		found := false
		items, _ := hooks[event].([]any)
		for itemIndex, item := range items {
			itemMap, _ := item.(map[string]any)
			handlers, _ := itemMap["hooks"].([]any)
			for hookIndex, handler := range handlers {
				handlerMap, _ := handler.(map[string]any)
				if got, _ := handlerMap["command"].(string); got == command {
					found = true
					identities = append(identities, codexHookIdentity{
						Event:    event,
						Identity: codexHookTrustIdentity(hooksPath, event, itemIndex, hookIndex),
						Command:  command,
					})
				}
			}
		}
		if !found {
			missing = append(missing, event)
		}
	}
	return identities, missing, nil
}

func codexHookTrustIdentity(hooksPath, event string, itemIndex, hookIndex int) string {
	return fmt.Sprintf("%s:%s:%d:%d", hooksPath, codexHookEventKey(event), itemIndex, hookIndex)
}

func codexHookTrustSection(identity string) string {
	return fmt.Sprintf("[hooks.state.%q]", identity)
}

func tomlHasSection(content, section string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == section {
			return true
		}
	}
	return false
}

func tomlSectionHasKey(content, section, wantKey string) bool {
	for _, line := range tomlSectionLines(content, section) {
		key, _, ok := tomlAssignment(strings.TrimSpace(line))
		if ok && key == wantKey {
			return true
		}
	}
	return false
}

func tomlSectionHasAssignment(content, section, wantKey, wantValue string) bool {
	for _, line := range tomlSectionLines(content, section) {
		key, value, ok := tomlAssignment(strings.TrimSpace(line))
		if ok && key == wantKey && value == wantValue {
			return true
		}
	}
	return false
}

func tomlSectionLines(content, section string) []string {
	var lines []string
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if isTOMLTableHeader(trimmed) {
			if inSection {
				break
			}
			inSection = trimmed == section
			continue
		}
		if inSection {
			lines = append(lines, line)
		}
	}
	return lines
}

func codexHookEventKey(event string) string {
	switch event {
	case "SessionStart":
		return "session_start"
	case "Stop":
		return "stop"
	default:
		return strings.ToLower(event)
	}
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
