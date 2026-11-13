package agents_test

import (
	"os"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-claude"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-codex"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-cursor"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-opencode"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-pi"
)

func TestExecutionKeysRemainStable(t *testing.T) {
	cases := []struct {
		agent   agents.Agent
		project string
		native  string
		want    string
	}{
		{agents.AgentCodex, "project-codex", "codex-session", "d35459bf7e66fce2fc9be610afe5ac283ccebdea92d0d764fd40424ec943e3dc"},
		{agents.AgentClaudeCode, "project-claude", "claude-session", "5d71b91a4702cb6d9d60208e0bb64a868216e6db4d56db26dc7ca46018caa6ad"},
		{agents.AgentCursor, "project-cursor", "conversation-1", "f22fac9b34f0d6f9b687824e0441cba06f17c7c9d2ffb0c6deff5418cffa6341"},
		{agents.AgentOpenCode, "project-opencode", "opencode-1", "01470ab095fbaa0643282603b401f674c1cb7d3a126de31e1e1324ef7a5ad7b3"},
		{agents.AgentPi, "project-pi", "pi-1", "3d3df5c8a8ae7b360a12a69270211896a1fb7184232fb98f6af584991cc50364"},
	}
	for _, tc := range cases {
		t.Run(string(tc.agent), func(t *testing.T) {
			got, err := agents.NewIdentity(tc.agent, tc.project, tc.native)
			if err != nil {
				t.Fatal(err)
			}
			if got.Execution != tc.want || got.Agent != tc.agent || got.Project != tc.project || got.NativeID != tc.native {
				t.Fatalf("execution identity changed: %+v", got)
			}
		})
	}
}

func TestCodexMetadataIdentity(t *testing.T) {
	adapter, ok := agents.ToolCallAdapterFor(agents.AgentCodex)
	if !ok {
		t.Fatal("Codex metadata adapter was not registered")
	}
	got, err := adapter.ParseToolCallMeta(map[string]any{
		"x-codex-turn-metadata": map[string]any{"session_id": "codex-session"},
	}, "project-codex")
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution != "d35459bf7e66fce2fc9be610afe5ac283ccebdea92d0d764fd40424ec943e3dc" {
		t.Fatalf("unexpected Codex execution identity: %s", got.Execution)
	}
	if _, err := adapter.ParseToolCallMeta(map[string]any{}, "project-codex"); err == nil {
		t.Fatal("missing Codex session ID must be rejected")
	}
}

func TestClaudeEnvironmentAndHook(t *testing.T) {
	environment, ok := agents.EnvironmentAdapterFor(agents.AgentClaudeCode)
	if !ok || environment.NativeIDEnvironmentVariable() != "CLAUDE_CODE_SESSION_ID" {
		t.Fatal("Claude environment contract changed")
	}
	hook, ok := agents.HookAdapterFor(agents.AgentClaudeCode)
	if !ok {
		t.Fatal("Claude hook adapter is missing")
	}
	got, err := hook.ParseHook([]byte("{\"session_id\":\"claude-session\"}"), "project-claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution != "5d71b91a4702cb6d9d60208e0bb64a868216e6db4d56db26dc7ca46018caa6ad" {
		t.Fatalf("unexpected Claude execution identity: %s", got.Execution)
	}
}

func TestCursorAndOpenCodeSideChannels(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	cases := []struct {
		agent   agents.Agent
		project string
		native  string
	}{
		{agents.AgentCursor, "cursor-project", "conversation-id"},
		{agents.AgentOpenCode, "opencode-project", "session-id"},
	}
	for _, tc := range cases {
		t.Run(string(tc.agent), func(t *testing.T) {
			adapter, ok := agents.SideChannelAdapterFor(tc.agent)
			if !ok {
				t.Fatalf("missing side-channel adapter for %s", tc.agent)
			}
			var path string
			switch tc.agent {
			case agents.AgentCursor:
				path = agents.CursorSideChannelPath(tc.project)
			case agents.AgentOpenCode:
				path = agents.OpenCodeSideChannelPath(tc.project)
			}
			if err := os.WriteFile(path, []byte(tc.native+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := adapter.ReadNativeID(tc.project)
			if err != nil || got != tc.native {
				t.Fatalf("read side-channel ID = %q, error = %v", got, err)
			}
		})
	}
}
