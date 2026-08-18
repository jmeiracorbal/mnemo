package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSetupUninstallArgsRequiresAgent(t *testing.T) {
	_, err := parseSetupUninstallArgs([]string{"--home=/home/test"}, func() (string, error) {
		t.Fatal("userHomeDir should not be called when --home is provided")
		return "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "missing --agent") {
		t.Fatalf("error = %v, want missing --agent", err)
	}
}

func TestParseSetupUninstallArgs(t *testing.T) {
	opts, err := parseSetupUninstallArgs([]string{"--agent=codex", "--home=/home/test"}, func() (string, error) {
		t.Fatal("userHomeDir should not be called when --home is provided")
		return "", nil
	})
	if err != nil {
		t.Fatalf("parse setup uninstall args: %v", err)
	}
	if opts.Agent != "codex" || opts.Home != "/home/test" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestUninstallSetupRemovesCodexFilesAndPreservesUserConfig(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "AGENTS.md"), "# Existing\n\nUser content.\n")
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.other]\ncommand = \"other\"\n")
	writeFile(t, filepath.Join(home, ".codex", "hooks.json"), `{"hooks":{"Stop":[{"matcher":"other","hooks":[{"type":"command","command":"/bin/other"}]}]}}`)

	if _, err := refreshSetup(setupRefreshOptions{Agent: "codex", Home: home, MnemoBin: "/bin/mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}
	removed, err := uninstallSetup(setupUninstallOptions{Agent: "codex", Home: home})
	if err != nil {
		t.Fatalf("uninstall setup: %v", err)
	}
	if len(removed) == 0 {
		t.Fatal("uninstall did not report removed paths")
	}

	agents := readTestFile(t, filepath.Join(home, ".codex", "AGENTS.md"))
	if !strings.Contains(agents, "# Existing\n\nUser content.") {
		t.Fatalf("user instructions were not preserved:\n%s", agents)
	}
	if strings.Contains(agents, "mnemo:start") {
		t.Fatalf("mnemo instructions were not removed:\n%s", agents)
	}

	config := readTestFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(config, "[mcp_servers.other]") {
		t.Fatalf("user MCP config was not preserved:\n%s", config)
	}
	if strings.Contains(config, "[mcp_servers.mnemo]") {
		t.Fatalf("mnemo MCP config was not removed:\n%s", config)
	}

	hooks := readTestFile(t, filepath.Join(home, ".codex", "hooks.json"))
	if !strings.Contains(hooks, "/bin/other") {
		t.Fatalf("user hook was not preserved:\n%s", hooks)
	}
	if strings.Contains(hooks, "session-start.sh") || strings.Contains(hooks, "stop.sh") {
		t.Fatalf("mnemo hooks were not removed:\n%s", hooks)
	}

	assertMissing(t, filepath.Join(home, ".codex", "hooks", "session-start.sh"))
	assertMissing(t, filepath.Join(home, ".codex", "hooks", "stop.sh"))
	assertMissing(t, filepath.Join(home, ".codex", "hooks", "mnemo-protocol.md"))
}

func TestUninstallSetupRemovesCursorConfigAndRuntimeFiles(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"other":{"command":"other"}}}`)
	writeFile(t, filepath.Join(home, ".cursor", "hooks.json"), `{"hooks":{"stop":[{"command":"/bin/other"}]}}`)

	if _, err := refreshSetup(setupRefreshOptions{Agent: "cursor", Home: home, MnemoBin: "/bin/mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}
	if _, err := uninstallSetup(setupUninstallOptions{Agent: "cursor", Home: home}); err != nil {
		t.Fatalf("uninstall setup: %v", err)
	}

	mcp := readTestFile(t, filepath.Join(home, ".cursor", "mcp.json"))
	if !strings.Contains(mcp, "other") || strings.Contains(mcp, "mnemo") {
		t.Fatalf("unexpected cursor MCP config:\n%s", mcp)
	}
	hooks := readTestFile(t, filepath.Join(home, ".cursor", "hooks.json"))
	if !strings.Contains(hooks, "/bin/other") || strings.Contains(hooks, "before-submit-prompt.sh") || strings.Contains(hooks, "stop.sh") {
		t.Fatalf("unexpected cursor hooks config:\n%s", hooks)
	}
	assertMissing(t, filepath.Join(home, ".cursor", "rules", "mnemo.mdc"))
	assertMissing(t, filepath.Join(home, ".cursor", "hooks", "before-submit-prompt.sh"))
	assertMissing(t, filepath.Join(home, ".cursor", "hooks", "stop.sh"))
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed: %v", path, err)
	}
}
