// Package cursor installs mnemo into a Cursor environment.
//
// Registration order:
//  1. Hook scripts → ~/.local/share/mnemo/cursor-hooks/
//  2. MCP server   → ~/.cursor/mcp.json
//  3. Hooks        → ~/.cursor/hooks.json (beforeSubmitPrompt, stop)
//  4. Rules doc    → ~/.cursor/rules/mnemo.mdc
package cursor

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

const mnemoRulesReference = "mnemo.mdc"

// Installer installs mnemo into a Cursor environment.
type Installer struct{}

// Install configures mnemo in the local Cursor environment.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	hooksDir := filepath.Join(home, ".local", "share", "mnemo", "cursor-hooks")
	cursorHooksJSON := filepath.Join(home, ".cursor", "hooks.json")
	cursorMCPJSON := filepath.Join(home, ".cursor", "mcp.json")
	cursorRulesDir := filepath.Join(home, ".cursor", "rules")

	if dryRun {
		fmt.Println("mnemo setup --cursor --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup --cursor")
	}
	fmt.Println("───────────────────────────────────────")

	if err := previewOrWriteHooks(hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	if err := previewOrRegisterMCP(cursorMCPJSON, dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectHooksJSON(cursorHooksJSON, hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks.json: %w", err)
	}

	if err := previewOrWriteRules(cursorRulesDir, dryRun); err != nil {
		return fmt.Errorf("rules: %w", err)
	}

	if !dryRun {
		fmt.Println("\nDone. Restart Cursor to activate mnemo.")
	}
	return nil
}

// Uninstall is not yet implemented.
func (i Installer) Uninstall() error {
	return fmt.Errorf("uninstall not yet implemented for Cursor")
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
	"before-submit-prompt.sh",
	"stop.sh",
}

func previewOrWriteHooks(dir string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[~/.local/share/mnemo/cursor-hooks/] — would write:\n")
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

// ─── MCP registration via ~/.cursor/mcp.json ─────────────────────────────────

func previewOrRegisterMCP(mcpPath string, dryRun bool) error {
	mnemoPath := plugin.ResolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[~/.cursor/mcp.json] — would add:\n  mnemo: %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

	// Use json.RawMessage to preserve all existing entries unchanged (e.g. url/transport fields).
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
		fmt.Println("✓ MCP server mnemo — already registered in ~/.cursor/mcp.json")
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
	fmt.Println("✓ MCP server mnemo registered in ~/.cursor/mcp.json")
	return nil
}

// ─── ~/.cursor/hooks.json ─────────────────────────────────────────────────────

type cursorHook struct {
	Command string `json:"command"`
}

type cursorHooksConfig struct {
	Version int                     `json:"version"`
	Hooks   map[string][]cursorHook `json:"hooks"`
}

func previewOrInjectHooksJSON(path, hooksDir string, dryRun bool) error {
	entries := map[string]string{
		"beforeSubmitPrompt": filepath.Join(hooksDir, "before-submit-prompt.sh"),
		"stop":               filepath.Join(hooksDir, "stop.sh"),
	}

	if dryRun {
		fmt.Printf("\n[~/.cursor/hooks.json] — would add hooks:\n")
		for event, script := range entries {
			fmt.Printf("  %s: %s\n", event, script)
		}
		return nil
	}

	config := cursorHooksConfig{Version: 1}
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}
	if config.Hooks == nil {
		config.Hooks = make(map[string][]cursorHook)
	}
	if config.Version == 0 {
		config.Version = 1
	}

	added := 0
	for event, script := range entries {
		if hookExists(config.Hooks[event], script) {
			continue
		}
		config.Hooks[event] = append(config.Hooks[event], cursorHook{Command: script})
		added++
	}

	if added == 0 {
		fmt.Println("✓ ~/.cursor/hooks.json — already up to date")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ ~/.cursor/hooks.json updated (%d hooks added)\n", added)
	return nil
}

func hookExists(hooks []cursorHook, command string) bool {
	for _, h := range hooks {
		if strings.EqualFold(h.Command, command) {
			return true
		}
	}
	return false
}

// ─── ~/.cursor/rules/mnemo.mdc ────────────────────────────────────────────────

func previewOrWriteRules(dir string, dryRun bool) error {
	rulesPath := filepath.Join(dir, mnemoRulesReference)

	if dryRun {
		fmt.Printf("\n[~/.cursor/rules/mnemo.mdc] — would write protocol doc\n")
		return nil
	}

	content, err := renderTemplate(templatesFS, "templates/mnemo.mdc", nil)
	if err != nil {
		return fmt.Errorf("mnemo.mdc template: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(rulesPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("✓ ~/.cursor/rules/mnemo.mdc written")
	return nil
}
