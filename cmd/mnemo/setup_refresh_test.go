package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSetupRefreshArgs(t *testing.T) {
	opts, err := parseSetupRefreshArgs(
		[]string{"--agent=codex", "--home=/home/test", "--mnemo-bin=/bin/mnemo"},
		func() (string, error) {
			t.Fatal("userHomeDir should not be called when --home is provided")
			return "", nil
		},
		func(string) (string, error) {
			t.Fatal("lookPath should not be called when --mnemo-bin is provided")
			return "", nil
		},
	)
	if err != nil {
		t.Fatalf("parse setup refresh args: %v", err)
	}
	if opts.Agent != "codex" || opts.Home != "/home/test" || opts.MnemoBin != "/bin/mnemo" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestRefreshSetupWritesCodexFiles(t *testing.T) {
	home := t.TempDir()
	updated, err := refreshSetup(setupRefreshOptions{Agent: "codex", Home: home, MnemoBin: "/bin/mnemo"})
	if err != nil {
		t.Fatalf("refresh setup: %v", err)
	}
	if len(updated) != 6 {
		t.Fatalf("updated paths = %d, want 6 (%v)", len(updated), updated)
	}

	config := readTestFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(config, `[mcp_servers.mnemo]`) || !strings.Contains(config, `command = "/bin/mnemo"`) {
		t.Fatalf("unexpected codex config:\n%s", config)
	}
	hooks := readTestFile(t, filepath.Join(home, ".codex", "hooks.json"))
	if !strings.Contains(hooks, filepath.Join(home, ".codex", "hooks", "session-start.sh")) {
		t.Fatalf("unexpected hooks config:\n%s", hooks)
	}
	assertExecutable(t, filepath.Join(home, ".codex", "hooks", "session-start.sh"))
	assertExecutable(t, filepath.Join(home, ".codex", "hooks", "stop.sh"))
	if got := readTestFile(t, filepath.Join(home, ".codex", "hooks", "mnemo-protocol.md")); !strings.Contains(got, "mnemo — Persistent Memory Protocol") {
		t.Fatalf("unexpected protocol: %q", got)
	}
}

func TestRefreshSetupPreservesExistingJSONConfig(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"other":{"command":"other"}}}`)

	if _, err := refreshSetup(setupRefreshOptions{Agent: "cursor", Home: home, MnemoBin: "mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}

	content := readTestFile(t, filepath.Join(home, ".cursor", "mcp.json"))
	if !strings.Contains(content, `"other"`) || !strings.Contains(content, `"mnemo"`) {
		t.Fatalf("merged config did not preserve existing server:\n%s", content)
	}
}

func TestRefreshSetupReplacesCodexMCPNestedTables(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, configPath, `[mcp_servers.mnemo]
command = "/old/mnemo"

[mcp_servers.mnemo.env]
STALE = "yes"

[mcp_servers.other]
command = "other"
`)

	if _, err := refreshSetup(setupRefreshOptions{Agent: "codex", Home: home, MnemoBin: "/bin/mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}

	content := readTestFile(t, configPath)
	if !strings.Contains(content, `[mcp_servers.mnemo]`) || !strings.Contains(content, `command = "/bin/mnemo"`) {
		t.Fatalf("mnemo MCP config was not refreshed:\n%s", content)
	}
	if strings.Contains(content, `[mcp_servers.mnemo.env]`) || strings.Contains(content, "STALE") {
		t.Fatalf("stale mnemo nested table was not removed:\n%s", content)
	}
	if !strings.Contains(content, `[mcp_servers.other]`) {
		t.Fatalf("unrelated MCP config was not preserved:\n%s", content)
	}
}

func TestRefreshSetupRestoresExistingScriptPermissions(t *testing.T) {
	home := t.TempDir()
	scriptPath := filepath.Join(home, ".codex", "hooks", "session-start.sh")
	writeFile(t, scriptPath, "#!/bin/sh\n")
	if err := os.Chmod(scriptPath, 0644); err != nil {
		t.Fatalf("chmod fixture: %v", err)
	}

	if _, err := refreshSetup(setupRefreshOptions{Agent: "codex", Home: home, MnemoBin: "mnemo"}); err != nil {
		t.Fatalf("refresh setup: %v", err)
	}

	assertExecutable(t, scriptPath)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("%s is not executable: %v", path, info.Mode())
	}
}
