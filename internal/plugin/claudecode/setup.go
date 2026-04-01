// Package claudecode installs mnemo into a Claude Code environment.
//
// When using the plugin path (recommended), the plugin manages hooks, MCP, and
// permissions via hooks.json and .mcp.json. This installer only handles what
// the plugin cannot do at install time:
//
//  1. Protocol doc → ~/.claude/mnemo.md
//  2. CLAUDE.md    → append @mnemo.md
package claudecode

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed scripts/*
var scriptsFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

const mnemoMDReference = "@mnemo.md"

// Installer installs mnemo into a Claude Code environment.
type Installer struct{}

// Install writes the memory protocol file and injects the @mnemo.md reference
// into ~/.claude/CLAUDE.md. Hooks, MCP, and permissions are managed by the
// plugin (hooks.json + .mcp.json) and must not be duplicated here.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")
	mnemoMDPath := filepath.Join(home, ".claude", "mnemo.md")

	if dryRun {
		fmt.Println("mnemo setup --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup")
	}
	fmt.Println("───────────────────────────────────────")

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
