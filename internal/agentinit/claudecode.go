package agentinit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

func claudeCodeLabel() string { return "Claude" }

func claudeConfigDir(home string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".claude")
}

func claudeCodeDetectionPaths(home string) []string {
	return []string{claudeConfigDir(home)}
}

func claudeCodeInstructionPath(home string) string {
	return filepath.Join(claudeConfigDir(home), "CLAUDE.md")
}

func claudeCodeSkillsDir(home string) string {
	return filepath.Join(claudeConfigDir(home), "skills")
}

// claudeCodeMCPPath returns the user-scope Claude Code config. With
// CLAUDE_CONFIG_DIR set, Claude keeps .claude.json inside that directory
// instead of the home directory.
func claudeCodeMCPPath(home string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, ".claude.json")
	}
	return filepath.Join(home, ".claude.json")
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
		Path:    claudeCodeMCPPath(home),
		Format:  "json",
		Content: claudeCodeMCPJSON(mnemoBin),
	}}
}

func claudeCodeMCPJSON(mnemoBin string) string {
	return prettyJSON(map[string]any{
		"mcpServers": map[string]any{
			"mnemo": map[string]any{
				"type":    "stdio",
				"command": mnemoBin,
				"args":    []string{"mcp", "--tools=agent"},
				"env": map[string]string{
					"MNEMO_AGENT":         AgentClaudeCode,
					"MNEMO_MCP_CLIENT":    AgentClaudeCode,
					"MNEMO_MCP_TRANSPORT": "stdio",
				},
			},
		},
	})
}

// claudeCodeHooksDir holds the hook scripts setup installs for Claude Code.
// Every hook whose command lives here belongs to mnemo.
func claudeCodeHooksDir(home string) string {
	return filepath.Join(claudeConfigDir(home), "hooks", "mnemo")
}

func claudeCodeSettingsPath(home string) string {
	return filepath.Join(claudeConfigDir(home), "settings.json")
}

var claudeCodeHookScripts = []string{"session-start.sh", "post-compact-resume.sh", "user-prompt-submit.sh", "pre-tool-use.sh"}

func claudeCodeRuntimeAssets(home string) []assetTarget {
	dir := claudeCodeHooksDir(home)
	targets := make([]assetTarget, 0, len(claudeCodeHookScripts)+2)
	for _, script := range claudeCodeHookScripts {
		targets = append(targets, assetTarget{Asset: "scripts/claudecode/hooks/" + script, Path: filepath.Join(dir, script), Mode: 0755})
	}
	for _, doc := range []string{"session-start-protocol.md", "post-compact-resume-protocol.md"} {
		targets = append(targets, assetTarget{Asset: "scripts/claudecode/hooks/" + doc, Path: filepath.Join(dir, doc), Mode: 0644})
	}
	return targets
}

// claudeCodeHooks returns the hook entries setup registers in Claude Code's
// user settings. Each runs its script in exec form, so the path needs no
// shell quoting.
func claudeCodeHooks(home string) map[string][]any {
	dir := claudeCodeHooksDir(home)
	hook := func(script string, timeout int, status string) map[string]any {
		h := map[string]any{"type": "command", "command": filepath.Join(dir, script), "args": []any{}, "timeout": timeout}
		if status != "" {
			h["statusMessage"] = status
		}
		return h
	}
	group := func(matcher string, h map[string]any) any {
		return map[string]any{"matcher": matcher, "hooks": []any{h}}
	}
	start := hook("session-start.sh", 10, "Loading mnemo memory...")
	return map[string][]any{
		"SessionStart": {
			group("startup", start),
			group("resume", start),
			group("clear", start),
			group("compact", hook("post-compact-resume.sh", 10, "Recovering mnemo context after compaction...")),
		},
		"UserPromptSubmit": {group("", hook("user-prompt-submit.sh", 5, ""))},
		"PreToolUse":       {group("mcp__mnemo__.*", hook("pre-tool-use.sh", 5, ""))},
	}
}

func claudeCodeInstallHooks(home string) ([]string, error) {
	path := claudeCodeSettingsPath(home)
	changed, err := upsertOwnedHooks(path, claudeCodeHooksDir(home), claudeCodeHooks(home))
	return appendChanged(path, changed, err)
}

func claudeCodeRemoveHooks(home string) ([]string, error) {
	path := claudeCodeSettingsPath(home)
	changed, err := upsertOwnedHooks(path, claudeCodeHooksDir(home), nil)
	return appendChanged(path, changed, err)
}

func claudeCodeUninstallConfig(home string) ([]string, error) {
	path := claudeCodeMCPPath(home)
	changed, err := removeMCPServer(path, "mcpServers", "mnemo")
	return appendChanged(path, changed, err)
}

func claudeCodeCheckInstructions(home string) Check {
	return checkInstructionFile("claudecode", claudeCodeInstructionPath(home), true)
}

func claudeCodeCheckMCP(home string) Check {
	path := claudeCodeMCPPath(home)
	return checkJSONMCPWithEnv(path, "mcp_config.claudecode", "claudecode", "env", "mcpServers", "mnemo")
}

func claudeCodeCheckRuntime(home string) Check {
	var paths []string
	for _, target := range claudeCodeRuntimeAssets(home) {
		paths = append(paths, target.Path)
	}
	check := checkFiles("claudecode", "runtime_files.claudecode", "Claude Code hooks installed", paths, true)
	if check.Status != "ok" {
		return check
	}
	expected := map[string][]string{}
	for event, groups := range claudeCodeHooks(home) {
		for _, group := range groups {
			for _, h := range group.(map[string]any)["hooks"].([]any) {
				expected[event] = append(expected[event], h.(map[string]any)["command"].(string))
			}
		}
	}
	return checkJSONHookCommands(claudeCodeSettingsPath(home), "runtime_files.claudecode", "claudecode", "Claude Code hooks installed", expected)
}

// claudeCodeLegacyPluginName identifies the Claude Code plugin and marketplace
// that earlier releases installed. Setup now owns hooks and the MCP server, so
// the plugin would register them a second time.
const claudeCodeLegacyPluginName = "mnemo@mnemo"

type claudeCodeLegacyInstall struct {
	Scope       string `json:"scope"`
	ProjectPath string `json:"projectPath"`
}

// claudeCodeLegacyPlugin reports the scopes where the legacy mnemo plugin is
// installed and whether its marketplace is still configured.
func claudeCodeLegacyPlugin(home string) ([]claudeCodeLegacyInstall, bool, error) {
	pluginsDir := filepath.Join(claudeConfigDir(home), "plugins")
	var installs []claudeCodeLegacyInstall
	data, err := os.ReadFile(filepath.Join(pluginsDir, "installed_plugins.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("read Claude Code plugin registry: %w", err)
	}
	if len(data) > 0 {
		var registry struct {
			Plugins map[string]json.RawMessage `json:"plugins"`
		}
		if err := json.Unmarshal(data, &registry); err != nil {
			return nil, false, fmt.Errorf("parse Claude Code plugin registry: %w", err)
		}
		if raw, ok := registry.Plugins[claudeCodeLegacyPluginName]; ok {
			if err := json.Unmarshal(raw, &installs); err != nil {
				var single claudeCodeLegacyInstall
				if err := json.Unmarshal(raw, &single); err != nil {
					return nil, false, fmt.Errorf("parse Claude Code mnemo plugin entry: %w", err)
				}
				installs = []claudeCodeLegacyInstall{single}
			}
		}
	}
	marketplace := false
	data, err = os.ReadFile(filepath.Join(pluginsDir, "known_marketplaces.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("read Claude Code marketplaces: %w", err)
	}
	if len(data) > 0 {
		var known map[string]json.RawMessage
		if err := json.Unmarshal(data, &known); err != nil {
			return nil, false, fmt.Errorf("parse Claude Code marketplaces: %w", err)
		}
		_, marketplace = known["mnemo"]
	}
	return installs, marketplace, nil
}

func claudeCodeLegacyFixCommands(installs []claudeCodeLegacyInstall, marketplace bool) []legacyCommand {
	var commands []legacyCommand
	for _, install := range installs {
		scope := install.Scope
		if scope == "" {
			scope = "user"
		}
		cmd := legacyCommand{Name: "claude", Args: []string{"plugin", "uninstall", claudeCodeLegacyPluginName, "--scope", scope}}
		if scope != "user" {
			cmd.Dir = install.ProjectPath
		}
		commands = append(commands, cmd)
	}
	if marketplace {
		commands = append(commands, legacyCommand{Name: "claude", Args: []string{"plugin", "marketplace", "remove", "mnemo"}})
	}
	return commands
}

func claudeCodeCheckLegacy(home string) Check {
	installs, marketplace, err := claudeCodeLegacyPlugin(home)
	if err != nil {
		return checkError("claudecode", "legacy_plugin.claudecode", err.Error(), filepath.Join(claudeConfigDir(home), "plugins"))
	}
	return legacyCheck("claudecode", "Claude Code", claudeCodeLegacyFixCommands(installs, marketplace), filepath.Join(claudeConfigDir(home), "plugins"))
}

func claudeCodeRemoveLegacy(home string) ([]string, error) {
	installs, marketplace, err := claudeCodeLegacyPlugin(home)
	if err != nil {
		return nil, err
	}
	return runLegacyCommands(home, claudeCodeLegacyFixCommands(installs, marketplace))
}
