package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmeiracorbal/mnemo-adapters/agents"
)

func TestApplyMCPCommandImportIsAtomic(t *testing.T) {
	src, err := New(FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Close() })

	sessionID, err := src.ResolveMCPInstanceSession("import-project", t.TempDir(), "instance-import", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.AddObservation(AddObservationParams{
		SessionID: sessionID, Project: "import-project", Type: "manual",
		Title: "imported obs", Content: "content for import test",
	}); err != nil {
		t.Fatal(err)
	}

	exported, err := src.Export()
	if err != nil {
		t.Fatal(err)
	}

	dst, err := New(FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dst.Close() })

	payload, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}

	first, err := dst.ApplyMCPCommand("import-cmd-1", "import", payload)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}

	var firstResult ImportResult
	if err := json.Unmarshal(first, &firstResult); err != nil {
		t.Fatalf("unmarshal first result: %v", err)
	}
	if firstResult.ObservationsImported != 1 {
		t.Fatalf("observations imported = %d, want 1", firstResult.ObservationsImported)
	}

	// Replay must return the cached result without re-importing.
	second, err := dst.ApplyMCPCommand("import-cmd-1", "import", payload)
	if err != nil {
		t.Fatalf("replay import: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("replayed result = %s, want %s", second, first)
	}

	// Verify the import and the command record committed atomically.
	rows, err := dst.q.ExportObservations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Title != "imported obs" {
		t.Fatalf("observations in dst = %d, want 1 with title 'imported obs'", len(rows))
	}
}

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
		Agent: agents.AgentPi, NativeID: nativeID, Project: project, Directory: directory,
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
	identity, err := agents.NewIdentity(agents.AgentPi, project, nativeID)
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
