package agentinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSnippetsForCodex(t *testing.T) {
	snippets, err := ConfigSnippets("/home/test", "/bin/mnemo", "codex")
	if err != nil {
		t.Fatalf("config snippets: %v", err)
	}
	if len(snippets) != 2 {
		t.Fatalf("snippets = %d, want 2", len(snippets))
	}
	if snippets[0].Path != "/home/test/.codex/config.toml" || !strings.Contains(snippets[0].Content, "[mcp_servers.mnemo]") || !strings.Contains(snippets[0].Content, `command = "/bin/mnemo"`) {
		t.Fatalf("unexpected mcp snippet: %+v", snippets[0])
	}
	if snippets[1].Path != "/home/test/.codex/hooks.json" || !strings.Contains(snippets[1].Content, "/home/test/.codex/hooks/session-start.sh") {
		t.Fatalf("unexpected hooks snippet: %+v", snippets[1])
	}
}

func TestConfigSnippetsForAll(t *testing.T) {
	var snippets []ConfigSnippet
	for _, agent := range SupportedAgents {
		agentSnippets, err := ConfigSnippets("/home/test", "mnemo", agent)
		if err != nil {
			t.Fatalf("config snippets for %s: %v", agent, err)
		}
		snippets = append(snippets, agentSnippets...)
	}
	if len(snippets) != 10 {
		t.Fatalf("snippets = %d, want 10", len(snippets))
	}
}

func TestConfigSnippetsIncludeAgentProvenanceEnvironment(t *testing.T) {
	for _, agent := range SupportedAgents {
		snippets, err := ConfigSnippets("/home/test", "/bin/mnemo", agent)
		if err != nil {
			t.Fatalf("config snippets for %s: %v", agent, err)
		}
		var mcpSnippet *ConfigSnippet
		for i := range snippets {
			if strings.Contains(snippets[i].Content, "mcp") && strings.Contains(snippets[i].Content, "mnemo") {
				mcpSnippet = &snippets[i]
				break
			}
		}
		if mcpSnippet == nil {
			t.Fatalf("%s: missing MCP snippet", agent)
		}
		if !strings.Contains(mcpSnippet.Content, "MNEMO_AGENT") || !strings.Contains(mcpSnippet.Content, agent) {
			t.Fatalf("%s: MCP snippet missing agent provenance env:\n%s", agent, mcpSnippet.Content)
		}
	}
}

func TestOpenCodeConfigUsesCurrentMCPServersShapeAndRemovesLegacyEntry(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeTestFile(t, configPath, `{
  "mcp": {
    "mnemo": {"type": "local", "command": ["/old/mnemo", "mcp"]},
    "other": {"type": "local", "command": ["/bin/other"]}
  }
}`)

	if _, err := Refresh(home, "/bin/mnemo", "opencode"); err != nil {
		t.Fatalf("refresh opencode: %v", err)
	}

	content := readTestFile(t, configPath)
	if strings.Contains(content, `"mnemo": {"type": "local", "command": ["/old/mnemo", "mcp"]}`) {
		t.Fatalf("legacy OpenCode MCP entry was not removed:\n%s", content)
	}
	if !strings.Contains(content, `"servers"`) || !strings.Contains(content, `"MNEMO_AGENT": "opencode"`) {
		t.Fatalf("OpenCode MCP config missing v2 servers shape or provenance env:\n%s", content)
	}
	if !strings.Contains(content, `"other"`) {
		t.Fatalf("unrelated OpenCode MCP config was not preserved:\n%s", content)
	}
}

func TestFxConfigUsesNativeMCPShape(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".fx", "mcp.json")
	writeTestFile(t, configPath, `{
  "mcp": {
    "other": {"command": ["/bin/other"]}
  }
}`)

	if _, err := Refresh(home, "/bin/mnemo", "fx"); err != nil {
		t.Fatalf("refresh fx: %v", err)
	}

	content := readTestFile(t, configPath)
	if !strings.Contains(content, `"mnemo"`) || !strings.Contains(content, `"command"`) || !strings.Contains(content, `"MNEMO_AGENT": "fx"`) {
		t.Fatalf("fx MCP config missing mnemo server or provenance env:\n%s", content)
	}
	if !strings.Contains(content, `"other"`) {
		t.Fatalf("unrelated fx MCP config was not preserved:\n%s", content)
	}
}

func TestPiConfigUsesMCPServersShape(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	writeTestFile(t, configPath, `{
  "mcpServers": {
    "other": {"command": "/bin/other"}
  }
}`)

	if _, err := Refresh(home, "/bin/mnemo", "pi"); err != nil {
		t.Fatalf("refresh pi: %v", err)
	}

	content := readTestFile(t, configPath)
	if !strings.Contains(content, `"mcpServers"`) || !strings.Contains(content, `"MNEMO_AGENT": "pi"`) || !strings.Contains(content, `"MNEMO_MCP_CLIENT": "pi"`) {
		t.Fatalf("Pi MCP config missing mnemo server or provenance env:\n%s", content)
	}
	if !strings.Contains(content, `"other"`) {
		t.Fatalf("unrelated Pi MCP config was not preserved:\n%s", content)
	}
}

func TestFxUninstallRemovesMCPConfig(t *testing.T) {
	home := t.TempDir()
	if _, err := Refresh(home, "/bin/mnemo", "fx"); err != nil {
		t.Fatalf("refresh fx: %v", err)
	}

	removed, err := Uninstall(home, "fx")
	if err != nil {
		t.Fatalf("uninstall fx: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("removed paths = %v, want instructions and MCP config", removed)
	}
	if _, err := os.Stat(filepath.Join(home, ".fx", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("fx AGENTS.md still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".fx", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("fx mcp.json still exists or stat failed: %v", err)
	}
}

func TestConfigSnippetsRejectsUnsupportedAgent(t *testing.T) {
	_, err := ConfigSnippets("/home/test", "mnemo", "future-agent")
	if err == nil || !strings.Contains(err.Error(), `unsupported agent "future-agent"`) {
		t.Fatalf("error = %v, want unsupported agent", err)
	}
}

func TestRefreshWritesCodexFiles(t *testing.T) {
	home := t.TempDir()
	updated, err := Refresh(home, "/bin/mnemo", "codex")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(updated) != 8 {
		t.Fatalf("updated paths = %d, want 8 (%v)", len(updated), updated)
	}

	config := readTestFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(config, `[mcp_servers.mnemo]`) || !strings.Contains(config, `command = "/bin/mnemo"`) {
		t.Fatalf("unexpected codex config:\n%s", config)
	}
	if !strings.Contains(config, "experimental_compact_prompt_file") {
		t.Fatalf("codex config missing compact prompt file:\n%s", config)
	}
	assertExecutable(t, filepath.Join(home, ".codex", "hooks", "session-start.sh"))
}

func TestRemoveCodexMCPConfigRemovesNestedTables(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".codex", "config.toml")
	writeTestFile(t, configPath, `[mcp_servers.mnemo]
command = "/bin/mnemo"

[mcp_servers.mnemo.env]
STALE = "yes"

[mcp_servers.other]
command = "other"
`)

	changed, err := removeCodexMCPConfig(configPath)
	if err != nil {
		t.Fatalf("remove codex MCP config: %v", err)
	}
	if !changed {
		t.Fatal("remove codex MCP config did not report a change")
	}

	content := readTestFile(t, configPath)
	if strings.Contains(content, `[mcp_servers.mnemo]`) || strings.Contains(content, `[mcp_servers.mnemo.env]`) || strings.Contains(content, "STALE") {
		t.Fatalf("mnemo table was not fully removed:\n%s", content)
	}
	if !strings.Contains(content, `[mcp_servers.other]`) {
		t.Fatalf("unrelated table was not preserved:\n%s", content)
	}
}

func TestRemoveRuntimePreservesRemovedPathsOnError(t *testing.T) {
	home := t.TempDir()
	firstPath := filepath.Join(home, ".codex", "hooks", "session-start.sh")
	secondPath := filepath.Join(home, ".codex", "hooks", "stop.sh")
	writeTestFile(t, firstPath, "#!/bin/sh\n")
	if err := os.MkdirAll(filepath.Join(secondPath, "child"), 0755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	removed, err := removeRuntime(home, "codex")
	if err == nil {
		t.Fatal("remove runtime unexpectedly succeeded")
	}
	if len(removed) != 1 || removed[0] != firstPath {
		t.Fatalf("removed = %v, want [%s]", removed, firstPath)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed: %v", firstPath, err)
	}
	if _, statErr := os.Stat(secondPath); statErr != nil {
		t.Fatalf("second path should remain after failed removal: %v", statErr)
	}
}

func TestLabelAndDetected(t *testing.T) {
	home := t.TempDir()
	if Label("claudecode") != "Claude" || Label("opencode") != "OpenCode" || Label("cursor") != "Cursor" {
		t.Fatalf("unexpected labels: %q %q %q", Label("claudecode"), Label("opencode"), Label("cursor"))
	}
	if Detected(home, "cursor") {
		t.Fatal("cursor should not be detected in empty home")
	}
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0755); err != nil {
		t.Fatal(err)
	}
	if !Detected(home, "cursor") {
		t.Fatal("cursor should be detected after creating ~/.cursor")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
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
