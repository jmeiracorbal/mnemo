package agentinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	mnemoassets "github.com/jmeiracorbal/mnemo"
	"github.com/jmeiracorbal/mnemo/internal/jsonmerge"
)

// ConfigSnippet is a file fragment to print or apply during setup.
type ConfigSnippet struct {
	Agent   string
	Path    string
	Format  string
	Content string
}

type assetTarget struct {
	Asset string
	Path  string
	Mode  os.FileMode
}

// Refresh writes global instructions, config, and runtime files for agent.
func Refresh(home, mnemoBin, agent string) ([]string, error) {
	var updated []string

	instructionsPath, err := InstallGlobalInstructions(home, agent)
	if err != nil {
		return nil, err
	}
	updated = append(updated, instructionsPath)

	snippets, err := ConfigSnippets(home, mnemoBin, agent)
	if err != nil {
		return nil, err
	}
	for _, snippet := range snippets {
		if err := applyConfig(snippet); err != nil {
			return nil, err
		}
		updated = append(updated, snippet.Path)
	}

	runtimeFiles, err := writeRuntime(home, agent)
	if err != nil {
		return nil, err
	}
	updated = append(updated, runtimeFiles...)
	return updated, nil
}

// Uninstall removes mnemo instructions, config, and runtime files for agent.
func Uninstall(home, agent string) ([]string, error) {
	var removed []string

	instructionsPath, changed, err := RemoveGlobalInstructions(home, agent)
	if err != nil {
		return removed, err
	}
	if changed {
		removed = append(removed, instructionsPath)
	}

	configFiles, err := uninstallConfig(home, agent)
	removed = append(removed, configFiles...)
	if err != nil {
		return removed, err
	}

	runtimeFiles, err := removeRuntime(home, agent)
	removed = append(removed, runtimeFiles...)
	if err != nil {
		return removed, err
	}
	return removed, nil
}

// ConfigSnippets returns the config fragments owned by agent.
func ConfigSnippets(home, mnemoBin, agent string) ([]ConfigSnippet, error) {
	switch agent {
	case "claudecode":
		return claudeCodeConfigSnippets(home, mnemoBin), nil
	case "cursor":
		return cursorConfigSnippets(home, mnemoBin), nil
	case "windsurf":
		return windsurfConfigSnippets(home, mnemoBin), nil
	case "codex":
		return codexConfigSnippets(home, mnemoBin), nil
	case "opencode":
		return openCodeConfigSnippets(home, mnemoBin), nil
	default:
		return nil, fmt.Errorf("unsupported agent %q for setup print-config", agent)
	}
}

func applyConfig(snippet ConfigSnippet) error {
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

func uninstallConfig(home, agent string) ([]string, error) {
	switch agent {
	case "claudecode":
		return claudeCodeUninstallConfig(home)
	case "cursor":
		return cursorUninstallConfig(home)
	case "windsurf":
		return windsurfUninstallConfig(home)
	case "codex":
		return codexUninstallConfig(home)
	case "opencode":
		return openCodeUninstallConfig(home)
	default:
		return nil, fmt.Errorf("unsupported agent %q for setup uninstall", agent)
	}
}

func runtimeAssets(agent string) ([]assetTarget, error) {
	switch agent {
	case "claudecode":
		return claudeCodeRuntimeAssets(), nil
	case "cursor":
		return cursorRuntimeAssets(), nil
	case "windsurf":
		return windsurfRuntimeAssets(), nil
	case "codex":
		return codexRuntimeAssets(), nil
	case "opencode":
		return openCodeRuntimeAssets(), nil
	default:
		return nil, fmt.Errorf("unsupported agent %q for setup runtime files", agent)
	}
}

func writeRuntime(home, agent string) ([]string, error) {
	targets, err := runtimeAssets(agent)
	if err != nil {
		return nil, err
	}
	return writeSetupAssets(home, targets)
}

func removeRuntime(home, agent string) ([]string, error) {
	targets, err := runtimeAssets(agent)
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

func writeSetupAssets(home string, targets []assetTarget) ([]string, error) {
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
		if err := os.Chmod(path, target.Mode); err != nil {
			return nil, err
		}
		updated = append(updated, path)
	}
	return updated, nil
}

func mcpServersJSON(mnemoBin string) string {
	return prettyJSON(map[string]any{
		"mcpServers": map[string]any{
			"mnemo": map[string]any{
				"command": mnemoBin,
				"args":    []string{"mcp", "--tools=agent"},
			},
		},
	})
}

func prettyJSON(v any) string {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(out) + "\n"
}

func appendChanged(path string, changed bool, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	if changed {
		return []string{path}, nil
	}
	return nil, nil
}
