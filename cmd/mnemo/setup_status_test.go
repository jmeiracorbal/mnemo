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

func TestParseSetupStatusArgsRejectsUnknown(t *testing.T) {
	if _, err := parseSetupStatusArgs([]string{"--bogus"}, func() (string, error) {
		return "/home/test", nil
	}); err == nil {
		t.Fatal("expected unknown argument error")
	}
}

func TestBuildSetupStatusReportReportsConfiguredCodex(t *testing.T) {
	home := t.TempDir()

	if _, err := agentinit.Refresh(home, "/bin/mnemo", "codex"); err != nil {
		t.Fatalf("refresh codex: %v", err)
	}
	appendCodexHookTrustForStatus(t, home)

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

func TestBuildSetupStatusReportReportsCodexHookReview(t *testing.T) {
	home := t.TempDir()

	if _, err := agentinit.Refresh(home, "/bin/mnemo", "codex"); err != nil {
		t.Fatalf("refresh codex: %v", err)
	}

	report := buildSetupStatusReport(setupStatusOptions{Agent: "codex", Home: home})
	if report.Status != "warning" || report.Summary.Warnings != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if got := report.Rows[0].Hooks; got != "review" {
		t.Fatalf("hooks = %q, want review", got)
	}
}

func TestBuildSetupStatusReportWarnsOnMissingMCPProvenanceEnv(t *testing.T) {
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
	if report.Status != "warning" || report.Summary.Warnings != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if got := report.Rows[0].MCP; got != "no" {
		t.Fatalf("mcp = %q, want no", got)
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

func TestBuildSetupStatusReportReportsConfiguredPi(t *testing.T) {
	home := t.TempDir()

	if _, err := agentinit.Refresh(home, "/bin/mnemo", "pi"); err != nil {
		t.Fatalf("refresh pi: %v", err)
	}

	report := buildSetupStatusReport(setupStatusOptions{Agent: "pi", Home: home})
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	row := report.Rows[0]
	if row.Agent != "Pi" || row.Detected != "yes" || row.MCP != "yes" || row.Hooks != "yes" || row.Instructions != "yes" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestBuildSetupStatusReportChecksClaudeCodeHooks(t *testing.T) {
	home := t.TempDir()
	if _, err := refreshSetup(setupRefreshOptions{Agent: "claudecode", Home: home, MnemoBin: "/bin/mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}

	report := buildSetupStatusReport(setupStatusOptions{Agent: "claudecode", Home: home})
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok: %+v", report.Status, report)
	}
	row := report.Rows[0]
	if row.Agent != "Claude" || row.Detected != "yes" || row.MCP != "yes" || row.Hooks != "yes" || row.Instructions != "yes" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestBuildSetupStatusReportWarnsOnClaudeCodeMissingHookCommands(t *testing.T) {
	home := t.TempDir()
	if _, err := refreshSetup(setupRefreshOptions{Agent: "claudecode", Home: home, MnemoBin: "/bin/mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}
	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{"theme":"dark"}`)

	report := buildSetupStatusReport(setupStatusOptions{Agent: "claudecode", Home: home})
	if report.Status != "warning" || report.Summary.Warnings != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if got := report.Rows[0].Hooks; got != "no" {
		t.Fatalf("hooks = %q, want no", got)
	}
}

func TestBuildSetupStatusReportWarnsWhenClaudeCodeHooksAreNotInstalled(t *testing.T) {
	home := t.TempDir()
	if _, err := agentinit.InstallGlobalInstructions(home, "claudecode"); err != nil {
		t.Fatalf("install instructions: %v", err)
	}
	writeFile(t, filepath.Join(home, ".claude.json"), claudeMCPFixture())

	report := buildSetupStatusReport(setupStatusOptions{Agent: "claudecode", Home: home})
	if report.Status != "warning" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if row := report.Rows[0]; row.MCP != "yes" || row.Hooks != "no" || row.Instructions != "yes" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestBuildSetupStatusReportCountsRuntimeErrors(t *testing.T) {
	home := t.TempDir()
	if _, err := refreshSetup(setupRefreshOptions{Agent: "claudecode", Home: home, MnemoBin: "/bin/mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}
	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{`)

	report := buildSetupStatusReport(setupStatusOptions{Agent: "claudecode", Home: home})
	if report.Status != "error" || report.Summary.Errors != 1 {
		t.Fatalf("status = %q errors = %d, want error/1", report.Status, report.Summary.Errors)
	}
	if got := report.Rows[0].Hooks; got != "error" {
		t.Fatalf("hooks = %q, want error", got)
	}
}

func appendCodexHookTrustForStatus(t *testing.T, home string) {
	t.Helper()
	configPath := filepath.Join(home, ".codex", "config.toml")
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	content := readTestFile(t, configPath)
	content += fmt.Sprintf(`
[hooks.state.%q]
trusted_hash = "sha256:test-session-start"

[hooks.state.%q]
trusted_hash = "sha256:test-stop"
`, hooksPath+":session_start:0:0", hooksPath+":stop:0:0")
	writeFile(t, configPath, content)
}

func claudeMCPFixture() string {
	return `{"mcpServers":{"mnemo":{"type":"stdio","command":"mnemo","args":["mcp","--tools=agent"],"env":{"MNEMO_AGENT":"claudecode","MNEMO_MCP_CLIENT":"claudecode","MNEMO_MCP_TRANSPORT":"stdio"}}}}`
}
