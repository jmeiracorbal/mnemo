package store

import (
	"encoding/json"
	"testing"
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
