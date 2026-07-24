package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupUninstallOptions struct {
	Agent string
	Home  string
}

func runSetupUninstall() {
	opts, err := parseSetupUninstallArgs(os.Args[3:], os.UserHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup uninstall: %v\n", err)
		os.Exit(1)
	}
	removed, err := uninstallSetup(opts)
	if err != nil {
		printSetupUninstallRemoved(removed)
		fmt.Fprintf(os.Stderr, "mnemo setup uninstall: %v\n", err)
		os.Exit(1)
	}
	printSetupUninstallRemoved(removed)
}

func printSetupUninstallRemoved(removed []string) {
	for _, path := range removed {
		fmt.Printf("mnemo setup uninstall: updated %s\n", path)
	}
}

func parseSetupUninstallArgs(args []string, userHomeDir func() (string, error)) (setupUninstallOptions, error) {
	var opts setupUninstallOptions
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.Agent == "" {
		return opts, fmt.Errorf("missing --agent=AGENT")
	}
	if opts.Home == "" {
		home, err := userHomeDir()
		if err != nil {
			return opts, fmt.Errorf("home: %w", err)
		}
		opts.Home = home
	}
	return opts, nil
}

func uninstallSetup(opts setupUninstallOptions) ([]string, error) {
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, agent := range agents {
		agentRemoved, err := uninstallAgentSetup(opts.Home, agent)
		removed = append(removed, agentRemoved...)
		if err != nil {
			return removed, fmt.Errorf("%s: %w", agent, err)
		}
	}
	return removed, nil
}

func uninstallAgentSetup(home, agent string) ([]string, error) {
	var removed []string

	instructionsPath, changed, err := agentinit.RemoveGlobalInstructions(home, agent)
	if err != nil {
		return removed, err
	}
	if changed {
		removed = append(removed, instructionsPath)
	}

	configFiles, err := uninstallAgentConfig(home, agent)
	removed = append(removed, configFiles...)
	if err != nil {
		return removed, err
	}

	runtimeFiles, err := uninstallAgentRuntimeFiles(home, agent)
	removed = append(removed, runtimeFiles...)
	if err != nil {
		return removed, err
	}
	return removed, nil
}

func uninstallAgentConfig(home, agent string) ([]string, error) {
	var removed []string
	appendIfChanged := func(path string, changed bool, err error) error {
		if err != nil {
			return err
		}
		if changed {
			removed = append(removed, path)
		}
		return nil
	}

	switch agent {
	case "claudecode":
		path := filepath.Join(home, ".claude", ".mcp.json")
		changed, err := removeMCPServer(path, "mcpServers", "mnemo")
		err = appendIfChanged(path, changed, err)
		return removed, err
	case "cursor":
		mcpPath := filepath.Join(home, ".cursor", "mcp.json")
		changed, err := removeMCPServer(mcpPath, "mcpServers", "mnemo")
		if err := appendIfChanged(mcpPath, changed, err); err != nil {
			return removed, err
		}
		hooksPath := filepath.Join(home, ".cursor", "hooks.json")
		hooksDir := filepath.Join(home, ".cursor", "hooks")
		changed, err = removeHookCommands(hooksPath, map[string]string{
			"beforeSubmitPrompt": filepath.Join(hooksDir, "before-submit-prompt.sh"),
			"stop":               filepath.Join(hooksDir, "stop.sh"),
		})
		err = appendIfChanged(hooksPath, changed, err)
		return removed, err
	case "windsurf":
		mcpPath := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
		changed, err := removeMCPServer(mcpPath, "mcpServers", "mnemo")
		if err := appendIfChanged(mcpPath, changed, err); err != nil {
			return removed, err
		}
		hooksPath := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
		hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
		changed, err = removeHookCommands(hooksPath, map[string]string{
			"pre_user_prompt":                       filepath.Join(hooksDir, "pre-user-prompt.sh"),
			"post_cascade_response_with_transcript": filepath.Join(hooksDir, "post-cascade-response.sh"),
		})
		err = appendIfChanged(hooksPath, changed, err)
		return removed, err
	case "codex":
		configPath := filepath.Join(home, ".codex", "config.toml")
		changed, err := removeCodexMCPConfig(configPath)
		if err := appendIfChanged(configPath, changed, err); err != nil {
			return removed, err
		}
		hooksPath := filepath.Join(home, ".codex", "hooks.json")
		hooksDir := filepath.Join(home, ".codex", "hooks")
		changed, err = removeHookCommands(hooksPath, map[string]string{
			"SessionStart": filepath.Join(hooksDir, "session-start.sh"),
			"Stop":         filepath.Join(hooksDir, "stop.sh"),
		})
		err = appendIfChanged(hooksPath, changed, err)
		return removed, err
	case "opencode":
		path := filepath.Join(home, ".config", "opencode", "opencode.json")
		changed, err := removeMCPServer(path, "mcp", "mnemo")
		err = appendIfChanged(path, changed, err)
		return removed, err
	default:
		return nil, fmt.Errorf("unsupported agent %q for setup uninstall", agent)
	}
}

func uninstallAgentRuntimeFiles(home, agent string) ([]string, error) {
	targets, err := setupRuntimeAssetTargets(agent)
	if err != nil {
		return nil, err
	}
	removed := make([]string, 0, len(targets))
	for _, target := range targets {
		path := filepath.Join(home, target.Path)
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		removed = append(removed, path)
	}
	return removed, nil
}

func removeMCPServer(path, rootKey, serverName string) (bool, error) {
	return updateJSONFile(path, func(root map[string]any) bool {
		servers, ok := root[rootKey].(map[string]any)
		if !ok {
			return false
		}
		if _, ok := servers[serverName]; !ok {
			return false
		}
		delete(servers, serverName)
		if len(servers) == 0 {
			delete(root, rootKey)
		}
		return true
	})
}

func removeHookCommands(path string, eventCommands map[string]string) (bool, error) {
	return updateJSONFile(path, func(root map[string]any) bool {
		hooks, ok := root["hooks"].(map[string]any)
		if !ok {
			return false
		}
		changed := false
		for event, command := range eventCommands {
			items, ok := hooks[event].([]any)
			if !ok {
				continue
			}
			kept := make([]any, 0, len(items))
			for _, item := range items {
				if containsCommand(item, command) {
					changed = true
					continue
				}
				kept = append(kept, item)
			}
			if len(kept) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = kept
			}
		}
		if len(hooks) == 0 {
			delete(root, "hooks")
		}
		return changed
	})
}

func updateJSONFile(path string, mutate func(map[string]any) bool) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var root map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	if !mutate(root) {
		return false, nil
	}
	if len(root) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0644); err != nil {
		return false, err
	}
	return true, nil
}

func containsCommand(v any, command string) bool {
	switch value := v.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "command" {
				if got, ok := item.(string); ok && got == command {
					return true
				}
			}
			if containsCommand(item, command) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if containsCommand(item, command) {
				return true
			}
		}
	}
	return false
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
