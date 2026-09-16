package store

import (
	"encoding/json"
	"testing"

	"github.com/jmeiracorbal/mnemo/adapters"
)

func TestApplyMCPCommandReturnsStoredResultWithoutRepeatingMutation(t *testing.T) {
	memory, err := New(FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	sessionID, err := memory.ResolveMCPInstanceSession("project-command", t.TempDir(), "instance-command", 1)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(AddObservationParams{SessionID: sessionID, Type: "manual", Title: "exactly once", Content: "a durable command must not repeat", Project: "project-command"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := memory.ApplyMCPCommand("command-1", "add_observation", payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := memory.ApplyMCPCommand("command-1", "add_observation", payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("replayed result = %s, want %s", second, first)
	}
	observations, err := memory.SessionObservations(sessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 {
		t.Fatalf("observations = %d, want 1", len(observations))
	}
}

func TestApplyMCPCommandExecutesPiToolAgainstCanonicalExecutionSession(t *testing.T) {
	memory, err := New(FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })

	project, nativeID, directory := "project-pi-tool", "pi-native-session", t.TempDir()
	payload, err := json.Marshal(AgentToolRequest{
		Agent: adapters.AgentPi, NativeID: nativeID, Project: project, Directory: directory,
		Tool: "mem_save", Arguments: map[string]any{"title": "Pi identity", "content": "The adapter must use Pi's native session."},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := memory.ApplyMCPCommand("pi-tool-command", "agent_tool", payload)
	if err != nil {
		t.Fatalf("execute Pi tool: %v", err)
	}
	second, err := memory.ApplyMCPCommand("pi-tool-command", "agent_tool", payload)
	if err != nil {
		t.Fatalf("replay Pi tool: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("replayed result = %s, want %s", second, first)
	}
	identity, err := adapters.NewIdentity(adapters.AgentPi, project, nativeID)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := memory.SessionObservations("execution-"+identity.Execution, 10)
	if err != nil {
		t.Fatalf("read canonical execution session: %v", err)
	}
	if len(observations) != 1 || observations[0].Title != "Pi identity" {
		t.Fatalf("observations = %+v, want one Pi observation", observations)
	}
}
