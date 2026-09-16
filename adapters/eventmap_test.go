package adapters

import (
	"encoding/json"
	"testing"
)

func TestEventMapForOpenCode(t *testing.T) {
	m, ok := EventMapFor(AgentOpenCode)
	if !ok {
		t.Fatal("OpenCode event map missing")
	}
	if m.Agent != AgentOpenCode {
		t.Fatalf("agent = %q, want %q", m.Agent, AgentOpenCode)
	}
	findEvent := func(nativeType string) *EventMapping {
		for i := range m.Events {
			if m.Events[i].NativeType == nativeType {
				return &m.Events[i]
			}
		}
		return nil
	}
	e := findEvent("session.created")
	if e == nil || e.MnemoType != "execution.started" || e.NativeIDPath == "" || e.PayloadContext != "directory" || e.LifecycleAction != LifecycleActionStartSession {
		t.Fatalf("session.created mapping = %+v", e)
	}
	e = findEvent("session.compacted")
	if e == nil || e.MnemoType != "session.compacted" || e.NativeIDPath == "" {
		t.Fatalf("session.compacted mapping = %+v", e)
	}
}

func TestEventMapForPi(t *testing.T) {
	m, ok := EventMapFor(AgentPi)
	if !ok {
		t.Fatal("Pi event map missing")
	}
	if m.Agent != AgentPi {
		t.Fatalf("agent = %q, want %q", m.Agent, AgentPi)
	}
	nativeToMnemo := make(map[string]string, len(m.Events))
	nativeToLifecycle := make(map[string]string, len(m.Events))
	for _, e := range m.Events {
		nativeToMnemo[e.NativeType] = e.MnemoType
		nativeToLifecycle[e.NativeType] = e.LifecycleAction
	}
	for native, want := range map[string]string{
		"session_start":    "execution.started",
		"session_shutdown": "execution.closed",
		"session_compact":  "session.compacted",
	} {
		if got := nativeToMnemo[native]; got != want {
			t.Fatalf("Pi event %q -> %q, want %q", native, got, want)
		}
	}
	if got := nativeToLifecycle["session_start"]; got != LifecycleActionStartSession {
		t.Fatalf("Pi session_start lifecycle_action = %q, want %q", got, LifecycleActionStartSession)
	}
}

func TestEventMapForUnknownAgent(t *testing.T) {
	if _, ok := EventMapFor(Agent("unknown")); ok {
		t.Fatal("unknown agent should have no event map")
	}
}

func TestAllEventMapsIncludesKnownAgents(t *testing.T) {
	all := AllEventMaps()
	seen := make(map[Agent]bool)
	for _, m := range all {
		seen[m.Agent] = true
	}
	for _, want := range []Agent{AgentOpenCode, AgentPi} {
		if !seen[want] {
			t.Fatalf("AllEventMaps missing %q", want)
		}
	}
}

func TestEventMapJSONRoundTrip(t *testing.T) {
	data, err := EventMapJSON(AgentOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	var result AgentEventMap
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Agent != AgentOpenCode {
		t.Fatalf("agent = %q, want %q", result.Agent, AgentOpenCode)
	}
	if len(result.Events) != len(agentEventMaps[AgentOpenCode].Events) {
		t.Fatalf("events count = %d, want %d", len(result.Events), len(agentEventMaps[AgentOpenCode].Events))
	}
}

func TestEventMapJSONAllAgents(t *testing.T) {
	data, err := EventMapJSON("")
	if err != nil {
		t.Fatal(err)
	}
	var result []AgentEventMap
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal all: %v", err)
	}
	if len(result) != len(agentEventMaps) {
		t.Fatalf("all maps count = %d, want %d", len(result), len(agentEventMaps))
	}
}

func TestEventMapJSONUnknownAgentErrors(t *testing.T) {
	if _, err := EventMapJSON(Agent("unknown")); err == nil {
		t.Fatal("unknown agent should error")
	}
}
