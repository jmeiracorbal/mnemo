package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

func TestParseSetupStatusArgs(t *testing.T) {
	opts, err := parseSetupStatusArgs([]string{"--json", "--agent=codex", "--home=/home/test"}, func() (string, error) {
		t.Fatal("userHomeDir should not be called when --home is provided")
		return "", nil
	})
	if err != nil {
		t.Fatalf("parse setup status args: %v", err)
	}
	if !opts.JSON || opts.Agent != "codex" || opts.Home != "/home/test" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestBuildSetupStatusReportReportsConfiguredCodex(t *testing.T) {
	home := t.TempDir()

	if _, err := agentinit.InstallGlobalInstructions(home, "codex"); err != nil {
		t.Fatalf("install instructions: %v", err)
	}
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.mnemo]\ncommand = \"mnemo\"\nargs = [\"mcp\", \"--tools=agent\"]\n")
	writeFile(t, filepath.Join(home, ".codex", "hooks.json"), `{"hooks":{"SessionStart":[],"Stop":[]}}`)
	writeExecutable(t, filepath.Join(home, ".codex", "hooks", "session-start.sh"))
	writeExecutable(t, filepath.Join(home, ".codex", "hooks", "stop.sh"))
	writeFile(t, filepath.Join(home, ".codex", "hooks", "mnemo-protocol.md"), "protocol")

	report := buildSetupStatusReport(setupStatusOptions{Agent: "codex", Home: home})
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(report.Rows))
	}
	row := report.Rows[0]
	if row.Agent != "Codex" || row.Detected != "yes" || row.MCP != "yes" || row.Hooks != "yes" || row.Instructions != "yes" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestBuildSetupStatusReportReportsMissingAgent(t *testing.T) {
	report := buildSetupStatusReport(setupStatusOptions{Agent: "cursor", Home: t.TempDir()})
	if report.Status != "warning" {
		t.Fatalf("status = %q, want warning", report.Status)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(report.Rows))
	}
	row := report.Rows[0]
	if row.Agent != "Cursor" || row.Detected != "no" || row.MCP != "no" || row.Hooks != "no" || row.Instructions != "no" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestBuildSetupStatusReportChecksClaudeCodePluginHooks(t *testing.T) {
	for name, registry := range map[string]string{
		"array":  `{"plugins":{"mnemo@mnemo":[{"installPath":"%s"}]}}`,
		"object": `{"plugins":{"mnemo@mnemo":{"installPath":"%s"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			installPath := filepath.Join(home, "plugin-cache", "mnemo")

			if _, err := agentinit.InstallGlobalInstructions(home, "claudecode"); err != nil {
				t.Fatalf("install instructions: %v", err)
			}
			writeFile(t, filepath.Join(home, ".claude", ".mcp.json"), `{"mcpServers":{"mnemo":{"command":"mnemo"}}}`)
			writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), fmt.Sprintf(registry, installPath))
			writeFile(t, filepath.Join(installPath, ".claude-plugin", "plugin.json"), `{"name":"mnemo"}`)
			writeFile(t, filepath.Join(installPath, "hooks", "hooks.json"), `{"hooks":{}}`)
			for _, script := range []string{
				"session-start.sh",
				"session-stop.sh",
				"subagent-stop.sh",
				"post-compact.sh",
				"post-compact-resume.sh",
				"post-file-edit.sh",
				"post-bash-git.sh",
			} {
				writeExecutable(t, filepath.Join(installPath, "scripts", script))
			}

			report := buildSetupStatusReport(setupStatusOptions{Agent: "claudecode", Home: home})
			if report.Status != "ok" {
				t.Fatalf("status = %q, want ok", report.Status)
			}
			row := report.Rows[0]
			if row.Agent != "Claude" || row.Detected != "yes" || row.MCP != "yes" || row.Hooks != "yes" || row.Instructions != "yes" {
				t.Fatalf("unexpected row: %+v", row)
			}
		})
	}
}

func TestBuildSetupStatusReportCountsRuntimeErrors(t *testing.T) {
	home := t.TempDir()

	if _, err := agentinit.InstallGlobalInstructions(home, "claudecode"); err != nil {
		t.Fatalf("install instructions: %v", err)
	}
	writeFile(t, filepath.Join(home, ".claude", ".mcp.json"), `{"mcpServers":{"mnemo":{"command":"mnemo"}}}`)
	writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), `{`)

	report := buildSetupStatusReport(setupStatusOptions{Agent: "claudecode", Home: home})
	if report.Status != "error" || report.Summary.Errors != 1 {
		t.Fatalf("status = %q errors = %d, want error/1", report.Status, report.Summary.Errors)
	}
	if got := report.Rows[0].Hooks; got != "error" {
		t.Fatalf("hooks = %q, want error", got)
	}
}
