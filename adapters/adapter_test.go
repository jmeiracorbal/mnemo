package adapters

import (
	"os"
	"testing"
)

func TestAdaptersExtractOnlyDocumentedNativeIDs(t *testing.T) {
	project := "project-uuid"
	tests := []struct {
		agent   Agent
		payload string
		wantID  string
	}{
		{AgentClaudeCode, `{"session_id":"claude-1"}`, "claude-1"},
		{AgentCodex, `{"session_id":"codex-1"}`, "codex-1"},
		{AgentCursor, `{"conversation_id":"cursor-1"}`, "cursor-1"},
	}
	for _, tt := range tests {
		t.Run(string(tt.agent), func(t *testing.T) {
			adapter, ok := HookAdapterFor(tt.agent)
			if !ok {
				t.Fatal("adapter missing")
			}
			identity, err := adapter.ParseHook([]byte(tt.payload), project)
			if err != nil {
				t.Fatal(err)
			}
			if identity.NativeID != tt.wantID || identity.Execution == "" {
				t.Fatalf("identity = %#v", identity)
			}
		})
	}
}

func TestOpenCodeDirectAdapterIdentity(t *testing.T) {
	adapter, ok := AdapterFor(AgentOpenCode)
	if !ok {
		t.Fatal("OpenCode adapter missing")
	}
	identity, err := adapter.Identity("project", "opencode-session")
	if err != nil {
		t.Fatal(err)
	}
	if identity.Agent != AgentOpenCode || identity.NativeID != "opencode-session" || identity.Execution == "" {
		t.Fatalf("identity = %#v", identity)
	}
	if _, ok := HookAdapterFor(AgentOpenCode); ok {
		t.Fatal("OpenCode must not claim a JSON hook adapter")
	}
}

func TestOpenCodeSideChannelReadsSessionID(t *testing.T) {
	project := "project-opencode-sc"
	nativeID := "opencode-session-sc-1"

	path := OpenCodeSideChannelPath(project)
	if err := os.WriteFile(path, []byte(nativeID), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	sc, ok := SideChannelAdapterFor(AgentOpenCode)
	if !ok {
		t.Fatal("opencode side-channel adapter missing")
	}
	gotID, err := sc.ReadNativeID(project)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != nativeID {
		t.Fatalf("ReadNativeID = %q, want %q", gotID, nativeID)
	}

	direct, _ := AdapterFor(AgentOpenCode)
	fromDirect, err := direct.Identity(project, gotID)
	if err != nil {
		t.Fatal(err)
	}
	fromSC, err := sc.Identity(project, gotID)
	if err != nil {
		t.Fatal(err)
	}
	if fromDirect != fromSC {
		t.Fatalf("side-channel identity = %#v, want direct identity %#v", fromSC, fromDirect)
	}
}

func TestOpenCodeSideChannelRejectsMissingFile(t *testing.T) {
	sc, ok := SideChannelAdapterFor(AgentOpenCode)
	if !ok {
		t.Fatal("opencode side-channel adapter missing")
	}
	if _, err := sc.ReadNativeID("project-no-file-exists-oc"); err == nil {
		t.Fatal("missing side-channel file was accepted")
	}
}

func TestIdentityIsStableAndScoped(t *testing.T) {
	first, err := NewIdentity(AgentClaudeCode, "project-a", "native-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewIdentity(AgentClaudeCode, "project-a", "native-a")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewIdentity(AgentClaudeCode, "project-b", "native-a")
	if err != nil {
		t.Fatal(err)
	}
	if first.Execution != second.Execution {
		t.Fatal("execution key is not stable")
	}
	if first.Execution == other.Execution {
		t.Fatal("execution key is not project-scoped")
	}
}

func TestAdaptersRejectMissingIdentityWithoutFallback(t *testing.T) {
	adapter, _ := HookAdapterFor(AgentCursor)
	if _, err := adapter.ParseHook([]byte(`{"generation_id":"g"}`), "project"); err == nil {
		t.Fatal("missing native id accepted")
	}
}

func TestCodexToolCallMetadataMatchesHookIdentity(t *testing.T) {
	project := "project-codex"
	hook, ok := HookAdapterFor(AgentCodex)
	if !ok {
		t.Fatal("Codex hook adapter missing")
	}
	fromHook, err := hook.ParseHook([]byte(`{"session_id":"codex-session"}`), project)
	if err != nil {
		t.Fatal(err)
	}
	toolCall, ok := ToolCallAdapterFor(AgentCodex)
	if !ok {
		t.Fatal("Codex tools/call adapter missing")
	}
	fromToolCall, err := toolCall.ParseToolCallMeta(map[string]any{
		"x-codex-turn-metadata": map[string]any{"session_id": "codex-session"},
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if fromToolCall != fromHook {
		t.Fatalf("tools/call identity = %#v, want %#v", fromToolCall, fromHook)
	}
}

func TestCursorSideChannelMatchesHookIdentity(t *testing.T) {
	project := "project-cursor-sc"
	nativeID := "cursor-conv-sc-1"

	path := CursorSideChannelPath(project)
	if err := os.WriteFile(path, []byte(nativeID), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	sc, ok := SideChannelAdapterFor(AgentCursor)
	if !ok {
		t.Fatal("cursor side-channel adapter missing")
	}
	gotID, err := sc.ReadNativeID(project)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != nativeID {
		t.Fatalf("ReadNativeID = %q, want %q", gotID, nativeID)
	}

	hook, _ := HookAdapterFor(AgentCursor)
	fromHook, err := hook.ParseHook([]byte(`{"conversation_id":"`+nativeID+`"}`), project)
	if err != nil {
		t.Fatal(err)
	}
	fromSC, err := sc.Identity(project, gotID)
	if err != nil {
		t.Fatal(err)
	}
	if fromHook != fromSC {
		t.Fatalf("side-channel identity = %#v, want hook identity %#v", fromSC, fromHook)
	}
}

func TestCursorSideChannelRejectsMissingFile(t *testing.T) {
	sc, ok := SideChannelAdapterFor(AgentCursor)
	if !ok {
		t.Fatal("cursor side-channel adapter missing")
	}
	if _, err := sc.ReadNativeID("project-no-file-exists"); err == nil {
		t.Fatal("missing side-channel file was accepted")
	}
}

func TestCodexToolCallMetadataRejectsMissingSessionID(t *testing.T) {
	adapter, ok := ToolCallAdapterFor(AgentCodex)
	if !ok {
		t.Fatal("Codex tools/call adapter missing")
	}
	if _, err := adapter.ParseToolCallMeta(map[string]any{
		"x-codex-turn-metadata": map[string]any{"thread_id": "thread-only"},
	}, "project"); err == nil {
		t.Fatal("missing Codex session_id accepted")
	}
}

func TestClaudeCodeEnvironmentCarrierMatchesHookIdentity(t *testing.T) {
	project := "project-claude"
	hook, ok := HookAdapterFor(AgentClaudeCode)
	if !ok {
		t.Fatal("Claude Code hook adapter missing")
	}
	fromHook, err := hook.ParseHook([]byte(`{"session_id":"claude-session"}`), project)
	if err != nil {
		t.Fatal(err)
	}
	environment, ok := EnvironmentAdapterFor(AgentClaudeCode)
	if !ok {
		t.Fatal("Claude Code environment adapter missing")
	}
	if variable := environment.NativeIDEnvironmentVariable(); variable != "CLAUDE_CODE_SESSION_ID" {
		t.Fatalf("environment variable = %q", variable)
	}
	fromEnvironment, err := environment.Identity(project, "claude-session")
	if err != nil {
		t.Fatal(err)
	}
	if fromEnvironment != fromHook {
		t.Fatalf("environment identity = %#v, want %#v", fromEnvironment, fromHook)
	}
}

func TestSupportIsExplicit(t *testing.T) {
	for _, agent := range []Agent{AgentClaudeCode, AgentCodex, AgentCursor, AgentOpenCode, AgentPi} {
		if !Supports(agent) {
			t.Fatalf("%s should have an explicit execution contract", agent)
		}
	}
	if _, err := NewIdentity(Agent("missing"), "project", "native"); err == nil {
		t.Fatal("unknown adapter accepted")
	}
}
