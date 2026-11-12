package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	events "github.com/jmeiracorbal/mnemo/internal/events"
)

func TestEventWireContract(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	event := events.Event{
		ID: "event-1",
		Type: events.EventExecutionStarted,
		Project: "project-a",
		Agent: agents.AgentCodex,
		NativeID: "session-a",
		Payload: json.RawMessage(`{"directory":"/project"}`),
		OccurredAt: now,
	}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "type", "project", "agent", "native_id", "payload", "occurred_at"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("serialized event lost field %q: %s", key, data)
		}
	}
	if got := string(decoded["agent"]); got != `"codex"` {
		t.Fatalf("serialized agent changed: %s", got)
	}
}

func TestEventValidationRejectsUnknownIdentity(t *testing.T) {
	event := events.Event{
		ID: "event-2", Type: events.EventExecutionStarted,
		Project: "project-a", Agent: agents.AgentCodex,
		Payload: json.RawMessage(`{}`), OccurredAt: time.Now(),
	}
	if err := event.Validate(); err == nil {
		t.Fatal("expected missing native ID to be rejected")
	}
}
