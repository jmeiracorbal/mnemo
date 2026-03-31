// Package windsurf installs mnemo into a Windsurf environment.
//
// Registration order:
//  1. Hook scripts → ~/.codeium/windsurf/hooks/
//  2. MCP server   → ~/.codeium/windsurf/mcp_config.json
//  3. Hooks        → ~/.codeium/windsurf/hooks.json
//  4. Global rules → ~/.codeium/windsurf/memories/global_rules.md
package windsurf

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jmeiracorbal/mnemo/internal/plugin"
)

//go:embed scripts/*
var scriptsFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

// Installer installs mnemo into a Windsurf environment.
type Installer struct{}

// Install configures mnemo in the local Windsurf environment.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
	windsurfHooksJSON := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	windsurfMCPJSON := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")

	if dryRun {
		fmt.Println("mnemo setup --windsurf --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup --windsurf")
	}
	fmt.Println("───────────────────────────────────────")

	if err := previewOrWriteHooks(hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	if err := previewOrRegisterMCP(windsurfMCPJSON, dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectHooksJSON(windsurfHooksJSON, hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks.json: %w", err)
	}

	windsurfMemoriesDir := filepath.Join(home, ".codeium", "windsurf", "memories")
	if err := previewOrWriteGlobalRules(windsurfMemoriesDir, dryRun); err != nil {
		return fmt.Errorf("global_rules.md: %w", err)
	}

	if !dryRun {
		fmt.Println("\nDone. Restart Windsurf to activate mnemo.")
	}
	return nil
}

// Uninstall is not yet implemented.
func (i Installer) Uninstall() error {
	return fmt.Errorf("uninstall not yet implemented for Windsurf")
}

// renderTemplate parses and executes an embedded template file.
func renderTemplate(fs embed.FS, path string, data any) (string, error) {
	raw, err := fs.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("embed %s: %w", path, err)
	}
	tmpl, err := template.New(filepath.Base(path)).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render %s: %w", path, err)
	}
	return buf.String(), nil
}

// ─── Hook scripts ─────────────────────────────────────────────────────────────

var hookScripts = []string{
	"pre-user-prompt.sh",
	"post-cascade-response.sh",
}

func previewOrWriteHooks(dir string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[~/.codeium/windsurf/hooks/] — would write:\n")
		for _, name := range hookScripts {
			fmt.Printf("  %s\n", name)
		}
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, name := range hookScripts {
		data, err := scriptsFS.ReadFile("scripts/" + name)
		if err != nil {
			return fmt.Errorf("embed script %s: %w", name, err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0755); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	fmt.Printf("✓ hook scripts written to %s\n", dir)
	return nil
}

// ─── MCP registration via ~/.codeium/windsurf/mcp_config.json ────────────────

func previewOrRegisterMCP(mcpPath string, dryRun bool) error {
	mnemoPath := plugin.ResolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[~/.codeium/windsurf/mcp_config.json] — would add:\n  mnemo: %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

	var config map[string]map[string]json.RawMessage
	data, err := os.ReadFile(mcpPath)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}
	if config == nil {
		config = make(map[string]map[string]json.RawMessage)
	}
	if config["mcpServers"] == nil {
		config["mcpServers"] = make(map[string]json.RawMessage)
	}

	if _, exists := config["mcpServers"]["mnemo"]; exists {
		fmt.Println("✓ MCP server mnemo — already registered in ~/.codeium/windsurf/mcp_config.json")
		return nil
	}

	entry := struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}{Command: mnemoPath, Args: []string{"mcp", "--tools=agent"}}
	raw, _ := json.Marshal(entry)
	config["mcpServers"]["mnemo"] = raw

	if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(mcpPath, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println("✓ MCP server mnemo registered in ~/.codeium/windsurf/mcp_config.json")
	return nil
}

// ─── ~/.codeium/windsurf/hooks.json ──────────────────────────────────────────

type windsurfHook struct {
	Command string `json:"command"`
}

type windsurfHooksConfig struct {
	Hooks map[string][]windsurfHook `json:"hooks"`
}

func previewOrInjectHooksJSON(path, hooksDir string, dryRun bool) error {
	entries := map[string]string{
		"pre_user_prompt":                      filepath.Join(hooksDir, "pre-user-prompt.sh"),
		"post_cascade_response_with_transcript": filepath.Join(hooksDir, "post-cascade-response.sh"),
	}

	if dryRun {
		fmt.Printf("\n[~/.codeium/windsurf/hooks.json] — would add hooks:\n")
		for event, script := range entries {
			fmt.Printf("  %s: %s\n", event, script)
		}
		return nil
	}

	var config windsurfHooksConfig
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}
	if config.Hooks == nil {
		config.Hooks = make(map[string][]windsurfHook)
	}

	added := 0
	for event, script := range entries {
		if hookExists(config.Hooks[event], script) {
			continue
		}
		config.Hooks[event] = append(config.Hooks[event], windsurfHook{Command: script})
		added++
	}

	if added == 0 {
		fmt.Println("✓ ~/.codeium/windsurf/hooks.json — already up to date")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ ~/.codeium/windsurf/hooks.json updated (%d hooks added)\n", added)
	return nil
}

func hookExists(hooks []windsurfHook, command string) bool {
	for _, h := range hooks {
		if strings.EqualFold(h.Command, command) {
			return true
		}
	}
	return false
}

// ─── ~/.codeium/windsurf/memories/global_rules.md ────────────────────────────

func previewOrWriteGlobalRules(dir string, dryRun bool) error {
	const marker = "## mnemo — Persistent Memory Protocol"

	if dryRun {
		rulesPath := filepath.Join(dir, "global_rules.md")
		data, err := os.ReadFile(rulesPath)
		if err == nil && strings.Contains(string(data), marker) {
			fmt.Println("\n[~/.codeium/windsurf/memories/global_rules.md] — already contains mnemo protocol, no change needed")
		} else {
			fmt.Printf("\n[~/.codeium/windsurf/memories/global_rules.md] — would append mnemo protocol\n")
		}
		return nil
	}

	rulesContent, err := renderTemplate(templatesFS, "templates/global_rules.md", nil)
	if err != nil {
		return fmt.Errorf("global_rules.md template: %w", err)
	}

	rulesPath := filepath.Join(dir, "global_rules.md")
	data, err := os.ReadFile(rulesPath)
	if err == nil && strings.Contains(string(data), marker) {
		fmt.Println("✓ ~/.codeium/windsurf/memories/global_rules.md — already up to date")
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Append to existing content or create new file.
	content := string(data)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content += "\n"
	}
	if len(content) > 0 {
		content += "\n"
	}
	content += rulesContent

	if err := os.WriteFile(rulesPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("✓ ~/.codeium/windsurf/memories/global_rules.md updated")
	return nil
}
