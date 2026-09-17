package mcp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/jmeiracorbal/mnemo/adapters"
	gomcp "github.com/mark3labs/mcp-go/mcp"
)

type recordingSessionBackend struct {
	MemoryBackend
	sessionID string
	project   string
	agent     adapters.Agent
	nativeID  string
}

func (b *recordingSessionBackend) ResolveMCPInstanceSession(project, directory, instance string, pid int) (string, error) {
	return b.sessionID, nil
}

func (b *recordingSessionBackend) BindExecutionSession(project string, agent adapters.Agent, nativeID, sessionID string) error {
	b.project = project
	b.agent = agent
	b.nativeID = nativeID
	return nil
}

func TestRuntimeUsesCodexToolCallSessionIdentity(t *testing.T) {
	runtime := Runtime{agent: adapters.AgentCodex}
	var request gomcp.CallToolRequest
	if err := json.Unmarshal([]byte(`{
		"params": {
			"name": "mem_save",
			"_meta": {
				"x-codex-turn-metadata": {
					"session_id": "codex-session",
					"thread_id": "codex-session"
				}
			}
		}
	}`), &request); err != nil {
		t.Fatalf("decode Codex tools/call: %v", err)
	}
	identity, binds, err := runtime.executionIdentity("project-codex", request)
	if err != nil {
		t.Fatal(err)
	}
	if !binds {
		t.Fatal("Codex tools/call identity did not require an execution binding")
	}
	want, err := adapters.NewIdentity(adapters.AgentCodex, "project-codex", "codex-session")
	if err != nil {
		t.Fatal(err)
	}
	if identity != want {
		t.Fatalf("identity = %#v, want %#v", identity, want)
	}
}

func TestRuntimeRejectsCodexToolCallWithoutSessionIdentity(t *testing.T) {
	runtime := Runtime{agent: adapters.AgentCodex}
	_, binds, err := runtime.executionIdentity("project-codex", gomcp.CallToolRequest{})
	if !binds {
		t.Fatal("Codex tools/call did not require an execution binding")
	}
	if err == nil {
		t.Fatal("Codex tools/call without metadata was accepted")
	}
}

func TestRuntimeUsesClaudeCodeEnvironmentSessionIdentity(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-session")
	runtime := Runtime{agent: adapters.AgentClaudeCode}
	identity, binds, err := runtime.executionIdentity("project-claude", gomcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !binds {
		t.Fatal("Claude Code environment carrier did not require an execution binding")
	}
	want, err := adapters.NewIdentity(adapters.AgentClaudeCode, "project-claude", "claude-session")
	if err != nil {
		t.Fatal(err)
	}
	if identity != want {
		t.Fatalf("identity = %#v, want %#v", identity, want)
	}
}

func TestRuntimeBindsClaudeCodeEnvironmentIdentityWhenResolvingSession(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-session")
	runtime := Runtime{instanceID: "mcp-instance", pid: 1, agent: adapters.AgentClaudeCode}
	backend := &recordingSessionBackend{sessionID: "mcp-session"}
	sessionID, err := runtime.resolveSession(backend, "project-claude", "/workspace", gomcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if sessionID != "mcp-session" {
		t.Fatalf("session ID = %q", sessionID)
	}
	if backend.project != "project-claude" || backend.agent != adapters.AgentClaudeCode || backend.nativeID != "claude-session" {
		t.Fatalf("binding = project=%q agent=%q nativeID=%q", backend.project, backend.agent, backend.nativeID)
	}
}

func TestRuntimeRejectsClaudeCodeWithoutEnvironmentSessionIdentity(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	runtime := Runtime{agent: adapters.AgentClaudeCode}
	_, binds, err := runtime.executionIdentity("project-claude", gomcp.CallToolRequest{})
	if !binds {
		t.Fatal("Claude Code environment carrier did not require an execution binding")
	}
	if err == nil {
		t.Fatal("Claude Code without CLAUDE_CODE_SESSION_ID was accepted")
	}
}

func TestRuntimeUsesCursorSideChannelIdentity(t *testing.T) {
	project := "project-cursor-mcp"
	nativeID := "cursor-conv-mcp-1"

	path := adapters.CursorSideChannelPath(project)
	if err := os.WriteFile(path, []byte(nativeID), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	runtime := Runtime{agent: adapters.AgentCursor}
	identity, binds, err := runtime.executionIdentity(project, gomcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !binds {
		t.Fatal("Cursor side-channel did not require an execution binding")
	}
	want, err := adapters.NewIdentity(adapters.AgentCursor, project, nativeID)
	if err != nil {
		t.Fatal(err)
	}
	if identity != want {
		t.Fatalf("identity = %#v, want %#v", identity, want)
	}
}

func TestRuntimeRejectsCursorWithoutSideChannelFile(t *testing.T) {
	runtime := Runtime{agent: adapters.AgentCursor}
	_, binds, err := runtime.executionIdentity("project-cursor-no-file", gomcp.CallToolRequest{})
	if !binds {
		t.Fatal("Cursor side-channel did not require an execution binding")
	}
	if err == nil {
		t.Fatal("Cursor missing side-channel file was accepted")
	}
}
