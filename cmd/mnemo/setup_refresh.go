package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mnemoassets "github.com/jmeiracorbal/mnemo"
	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	"github.com/jmeiracorbal/mnemo/internal/jsonmerge"
)

type setupRefreshOptions struct {
	Agent    string
	Home     string
	MnemoBin string
}

func runSetupRefresh() {
	opts, err := parseSetupRefreshArgs(os.Args[3:], os.UserHomeDir, exec.LookPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup refresh: %v\n", err)
		os.Exit(1)
	}
	updated, err := refreshSetup(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup refresh: %v\n", err)
		os.Exit(1)
	}
	for _, path := range updated {
		fmt.Printf("mnemo setup refresh: updated %s\n", path)
	}
}

func parseSetupRefreshArgs(args []string, userHomeDir func() (string, error), lookPath func(string) (string, error)) (setupRefreshOptions, error) {
	opts := setupRefreshOptions{Agent: "all"}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		case strings.HasPrefix(arg, "--mnemo-bin="):
			opts.MnemoBin = arg[len("--mnemo-bin="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.Agent == "" {
		opts.Agent = "all"
	}
	if opts.Home == "" {
		home, err := userHomeDir()
		if err != nil {
			return opts, fmt.Errorf("home: %w", err)
		}
		opts.Home = home
	}
	if opts.MnemoBin == "" {
		if path, err := lookPath("mnemo"); err == nil && strings.TrimSpace(path) != "" {
			opts.MnemoBin = path
		} else {
			opts.MnemoBin = "mnemo"
		}
	}
	return opts, nil
}

func refreshSetup(opts setupRefreshOptions) ([]string, error) {
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return nil, err
	}
	var updated []string
	for _, agent := range agents {
		agentUpdated, err := refreshAgentSetup(opts.Home, opts.MnemoBin, agent)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", agent, err)
		}
		updated = append(updated, agentUpdated...)
	}
	return updated, nil
}

func refreshAgentSetup(home, mnemoBin, agent string) ([]string, error) {
	var updated []string

	instructionsPath, err := agentinit.InstallGlobalInstructions(home, agent)
	if err != nil {
		return nil, err
	}
	updated = append(updated, instructionsPath)

	snippets, err := setupConfigSnippetsForAgent(home, mnemoBin, agent)
	if err != nil {
		return nil, err
	}
	for _, snippet := range snippets {
		if err := applySetupConfigSnippet(snippet); err != nil {
			return nil, err
		}
		updated = append(updated, snippet.Path)
	}

	runtimeFiles, err := refreshAgentRuntimeFiles(home, agent)
	if err != nil {
		return nil, err
	}
	updated = append(updated, runtimeFiles...)
	return updated, nil
}

func applySetupConfigSnippet(snippet setupConfigSnippet) error {
	switch snippet.Format {
	case "json":
		var patch any
		if err := json.Unmarshal([]byte(snippet.Content), &patch); err != nil {
			return fmt.Errorf("parse generated JSON for %s: %w", snippet.Path, err)
		}
		if _, err := jsonmerge.MergeValue(snippet.Path, patch); err != nil {
			return err
		}
	case "toml":
		if err := upsertCodexMCPConfig(snippet.Path, snippet.Content); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported config format %q for %s", snippet.Format, snippet.Path)
	}
	return nil
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
		if trimmed == "[mcp_servers.mnemo]" {
			out = append(out, strings.TrimRight(section, "\n"))
			skipping = true
			replaced = true
			continue
		}
		if skipping && strings.HasPrefix(trimmed, "[") {
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

func refreshAgentRuntimeFiles(home, agent string) ([]string, error) {
	switch agent {
	case "claudecode":
		// Claude Code hooks are managed by the Claude plugin installation.
		return nil, nil
	case "cursor":
		return writeSetupAssets(home, []setupAssetTarget{
			{Asset: "scripts/cursor/hooks/before-submit-prompt.sh", Path: filepath.Join(".cursor", "hooks", "before-submit-prompt.sh"), Mode: 0755},
			{Asset: "scripts/cursor/hooks/stop.sh", Path: filepath.Join(".cursor", "hooks", "stop.sh"), Mode: 0755},
		})
	case "windsurf":
		return writeSetupAssets(home, []setupAssetTarget{
			{Asset: "scripts/windsurf/hooks/pre-user-prompt.sh", Path: filepath.Join(".codeium", "windsurf", "hooks", "pre-user-prompt.sh"), Mode: 0755},
			{Asset: "scripts/windsurf/hooks/post-cascade-response.sh", Path: filepath.Join(".codeium", "windsurf", "hooks", "post-cascade-response.sh"), Mode: 0755},
		})
	case "codex":
		return writeSetupAssets(home, []setupAssetTarget{
			{Asset: "scripts/codex/hooks/session-start.sh", Path: filepath.Join(".codex", "hooks", "session-start.sh"), Mode: 0755},
			{Asset: "scripts/codex/hooks/stop.sh", Path: filepath.Join(".codex", "hooks", "stop.sh"), Mode: 0755},
			{Asset: "scripts/codex/hooks/mnemo-protocol.md", Path: filepath.Join(".codex", "hooks", "mnemo-protocol.md"), Mode: 0644},
		})
	case "opencode":
		return writeSetupAssets(home, []setupAssetTarget{
			{Asset: "scripts/opencode/plugins/mnemo.ts", Path: filepath.Join(".config", "opencode", "plugins", "mnemo.ts"), Mode: 0644},
			{Asset: "scripts/opencode/plugins/mnemo-protocol.md", Path: filepath.Join(".config", "opencode", "plugins", "mnemo-protocol.md"), Mode: 0644},
		})
	default:
		return nil, fmt.Errorf("unsupported agent %q for setup refresh", agent)
	}
}

type setupAssetTarget struct {
	Asset string
	Path  string
	Mode  os.FileMode
}

func writeSetupAssets(home string, targets []setupAssetTarget) ([]string, error) {
	updated := make([]string, 0, len(targets))
	for _, target := range targets {
		data, err := mnemoassets.SetupAssets.ReadFile(target.Asset)
		if err != nil {
			return nil, fmt.Errorf("read embedded asset %s: %w", target.Asset, err)
		}
		path := filepath.Join(home, target.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, target.Mode); err != nil {
			return nil, err
		}
		updated = append(updated, path)
	}
	return updated, nil
}
