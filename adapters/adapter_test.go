package adapters

import (
	"errors"
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
		{AgentWindsurf, `{"trajectory_id":"windsurf-1"}`, "windsurf-1"},
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

func TestOpenCodeUsesTheCommonDirectAdapter(t *testing.T) {
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
	if _, err := NewIdentity(AgentPi, "project", "native"); err == nil {
		t.Fatal("unsupported agent accepted")
	}
}

func TestSupportIsExplicit(t *testing.T) {
	for _, agent := range []Agent{AgentClaudeCode, AgentCodex, AgentCursor, AgentWindsurf, AgentOpenCode} {
		if !Supports(agent) {
			t.Fatalf("%s should have an explicit execution contract", agent)
		}
	}
	for _, agent := range []Agent{AgentFx, AgentPi} {
		adapter, ok := AdapterFor(agent)
		if !ok || adapter.Agent() != agent {
			t.Fatalf("%s must have its own adapter", agent)
		}
		if Supports(agent) {
			t.Fatalf("%s must not be approximated", agent)
		}
		if _, err := adapter.Identity("project", "native"); !errors.Is(err, ErrExecutionIdentityUnsupported) {
			t.Fatalf("%s error = %v, want ErrExecutionIdentityUnsupported", agent, err)
		}
	}
}
