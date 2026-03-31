// Package claudecode installs mnemo into a Claude Code environment.
//
// Registration order:
//  1. Hook scripts → ~/.claude/hooks/
//  2. MCP server   → `claude mcp add -s user mnemo` (writes to ~/.claude.json)
//  3. Hooks        → ~/.claude/settings.json (SessionStart, Stop, SubagentStop)
//  4. Permissions  → ~/.claude/settings.json permissions.allow
//  5. Protocol doc → ~/.claude/mnemo.md
//  6. CLAUDE.md    → append @mnemo.md
//
// NOTE: mcpServers in settings.json is NOT used by Claude Code for standalone
// servers — the correct mechanism is `claude mcp add` which writes to ~/.claude.json.
package claudecode

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jmeiracorbal/mnemo/internal/plugin"
)

//go:embed scripts/*
var scriptsFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

const mnemoMDReference = "@mnemo.md"

// mnemoTools is the list of MCP tool names to add to permissions.allow.
var mnemoTools = []string{
	"mcp__mnemo__mem_save",
	"mcp__mnemo__mem_search",
	"mcp__mnemo__mem_context",
	"mcp__mnemo__mem_session_summary",
	"mcp__mnemo__mem_session_start",
	"mcp__mnemo__mem_session_end",
	"mcp__mnemo__mem_get_observation",
	"mcp__mnemo__mem_suggest_topic_key",
	"mcp__mnemo__mem_capture_passive",
	"mcp__mnemo__mem_save_prompt",
	"mcp__mnemo__mem_update",
}

// Installer installs mnemo into a Claude Code environment.
type Installer struct{}

// Install configures mnemo in the local Claude Code environment.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")
	mnemoMDPath := filepath.Join(home, ".claude", "mnemo.md")
	hooksDir := filepath.Join(home, ".claude", "hooks")

	if dryRun {
		fmt.Println("mnemo setup --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup")
	}
	fmt.Println("───────────────────────────────────────")

	if err := previewOrWriteHooks(hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	if err := previewOrRegisterMCP(dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectSettings(settingsPath, hooksDir, dryRun); err != nil {
		return fmt.Errorf("settings.json: %w", err)
	}

	protocolDoc, err := renderTemplate(templatesFS, "templates/mnemo.md", nil)
	if err != nil {
		return fmt.Errorf("mnemo.md template: %w", err)
	}

	if dryRun {
		fmt.Printf("\n[~/.claude/mnemo.md]\n%s\n", protocolDoc)
	} else {
		if err := os.WriteFile(mnemoMDPath, []byte(protocolDoc), 0644); err != nil {
			return fmt.Errorf("mnemo.md: %w", err)
		}
		fmt.Println("✓ ~/.claude/mnemo.md written")
	}

	if err := previewOrInjectCLAUDEMD(claudeMDPath, dryRun); err != nil {
		return fmt.Errorf("CLAUDE.md: %w", err)
	}

	if !dryRun {
		fmt.Println("\nDone. Restart Claude Code to activate mnemo.")
	}
	return nil
}

// Uninstall is not yet implemented.
func (i Installer) Uninstall() error {
	return fmt.Errorf("uninstall not yet implemented for Claude Code")
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
	"session-start.sh",
	"session-stop.sh",
	"subagent-stop.sh",
	"post-compact.sh",
	"post-compact-resume.sh",
}

// hookProtocols are template files written alongside the scripts so hooks can
// cat them at runtime without embedding text content in the scripts themselves.
var hookProtocols = []string{
	"session-start-protocol.md",
	"post-compact-protocol-header.md",
	"post-compact-protocol-footer.md",
	"post-compact-resume-protocol.md",
}

func previewOrWriteHooks(dir string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[~/.claude/hooks/] — would write:\n")
		for _, name := range hookScripts {
			fmt.Printf("  %s\n", name)
		}
		for _, name := range hookProtocols {
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
		if err := os.WriteFile(filepath.Join(dir, name), data, 0755); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	for _, name := range hookProtocols {
		content, err := renderTemplate(templatesFS, "templates/"+name, nil)
		if err != nil {
			return fmt.Errorf("protocol template %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	fmt.Printf("✓ hook scripts written to %s\n", dir)
	return nil
}

// ─── MCP server registration via claude CLI ───────────────────────────────────

func previewOrRegisterMCP(dryRun bool) error {
	mnemoPath := plugin.ResolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[claude mcp add] would run:\n  claude mcp add -s user mnemo -- %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

	listOut, _ := exec.Command("claude", "mcp", "list").CombinedOutput()
	if strings.Contains(string(listOut), "mnemo:") {
		fmt.Println("✓ MCP server mnemo — already registered")
		return nil
	}

	cmd := exec.Command("claude", "mcp", "add", "-s", "user", "mnemo", "--",
		mnemoPath, "mcp", "--tools=agent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude mcp add failed: %w\n%s", err, string(out))
	}
	fmt.Println("✓ MCP server mnemo registered via claude mcp add")
	return nil
}

// ─── settings.json (hooks + permissions only) ─────────────────────────────────

func previewOrInjectSettings(path, hooksDir string, dryRun bool) error {
	config, err := plugin.ReadJSON(path)
	if err != nil {
		return err
	}

	removeStaleMCPEntry(config)

	if err := injectHooks(config, hooksDir); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	if err := injectPermissions(config); err != nil {
		return fmt.Errorf("permissions: %w", err)
	}

	if dryRun {
		out, _ := json.MarshalIndent(config, "", "  ")
		fmt.Printf("\n[~/.claude/settings.json]\n%s\n", string(out))
		return nil
	}

	if err := plugin.WriteJSON(path, config); err != nil {
		return err
	}
	fmt.Println("✓ ~/.claude/settings.json updated (hooks + permissions)")
	return nil
}

func removeStaleMCPEntry(config map[string]json.RawMessage) {
	raw, ok := config["mcpServers"]
	if !ok {
		return
	}
	var mcpServers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mcpServers); err != nil {
		return
	}
	if _, exists := mcpServers["mnemo"]; !exists {
		return
	}
	delete(mcpServers, "mnemo")
	if len(mcpServers) == 0 {
		delete(config, "mcpServers")
		return
	}
	encoded, _ := json.Marshal(mcpServers)
	config["mcpServers"] = encoded
}

func injectHooks(config map[string]json.RawMessage, hooksDir string) error {
	var hooks map[string]json.RawMessage
	if raw, ok := config["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return err
		}
	} else {
		hooks = make(map[string]json.RawMessage)
	}

	sessionStartScript := filepath.Join(hooksDir, "session-start.sh")
	postCompactResumeScript := filepath.Join(hooksDir, "post-compact-resume.sh")
	postCompactionScript := filepath.Join(hooksDir, "post-compact.sh")

	// SessionStart: separate matchers per source so compact gets its own recovery script.
	if _, exists := hooks["SessionStart"]; !exists {
		sessionStartHook := []map[string]interface{}{
			{"matcher": "startup", "hooks": []map[string]interface{}{{"type": "command", "command": sessionStartScript}}},
			{"matcher": "resume", "hooks": []map[string]interface{}{{"type": "command", "command": sessionStartScript}}},
			{"matcher": "clear", "hooks": []map[string]interface{}{{"type": "command", "command": sessionStartScript}}},
			{"matcher": "compact", "hooks": []map[string]interface{}{{"type": "command", "command": postCompactResumeScript}}},
		}
		raw, err := json.Marshal(sessionStartHook)
		if err != nil {
			return err
		}
		hooks["SessionStart"] = raw
	}

	type hookEntry struct {
		key    string
		script string
	}
	simpleEntries := []hookEntry{
		{"Stop", filepath.Join(hooksDir, "session-stop.sh")},
		{"SubagentStop", filepath.Join(hooksDir, "subagent-stop.sh")},
		{"PostCompact", postCompactionScript},
	}

	for _, e := range simpleEntries {
		if _, exists := hooks[e.key]; exists {
			continue
		}
		hook := []map[string]interface{}{
			{
				"matcher": "",
				"hooks": []map[string]interface{}{
					{"type": "command", "command": e.script},
				},
			},
		}
		raw, err := json.Marshal(hook)
		if err != nil {
			return err
		}
		hooks[e.key] = raw
	}

	encoded, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	config["hooks"] = encoded
	return nil
}

func injectPermissions(config map[string]json.RawMessage) error {
	var permissions map[string]json.RawMessage
	if raw, ok := config["permissions"]; ok {
		if err := json.Unmarshal(raw, &permissions); err != nil {
			return err
		}
	} else {
		permissions = make(map[string]json.RawMessage)
	}

	var allow []string
	if raw, ok := permissions["allow"]; ok {
		if err := json.Unmarshal(raw, &allow); err != nil {
			return err
		}
	}

	existing := make(map[string]bool, len(allow))
	for _, t := range allow {
		existing[t] = true
	}
	for _, tool := range mnemoTools {
		if !existing[tool] {
			allow = append(allow, tool)
		}
	}

	raw, err := json.Marshal(allow)
	if err != nil {
		return err
	}
	permissions["allow"] = raw

	encoded, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	config["permissions"] = encoded
	return nil
}

// ─── CLAUDE.md ────────────────────────────────────────────────────────────────

func previewOrInjectCLAUDEMD(path string, dryRun bool) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte{}
	} else if err != nil {
		return err
	}

	content := string(data)
	if strings.Contains(content, mnemoMDReference) {
		if dryRun {
			fmt.Printf("\n[~/.claude/CLAUDE.md] — already contains %s, no change needed\n", mnemoMDReference)
		} else {
			fmt.Println("✓ ~/.claude/CLAUDE.md — already up to date")
		}
		return nil
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	newContent := content + mnemoMDReference + "\n"

	if dryRun {
		fmt.Printf("\n[~/.claude/CLAUDE.md] — would append:\n  %s\n", mnemoMDReference)
		return nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return err
	}
	fmt.Println("✓ ~/.claude/CLAUDE.md updated")
	return nil
}
