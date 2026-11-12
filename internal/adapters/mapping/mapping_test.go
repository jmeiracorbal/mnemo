package mapping_test

import (
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-opencode"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-pi"
	"github.com/jmeiracorbal/mnemo/internal/adapters/mapping"
)

func TestAgentEventMappingsAreUnchanged(t *testing.T) {
	cases := []struct {
		agent      agents.Agent
		native     string
		normalized string
	}{
		{agents.AgentOpenCode, "session.created", "execution.started"},
		{agents.AgentOpenCode, "session.compacted", "session.compacted"},
		{agents.AgentPi, "session_start", "execution.started"},
		{agents.AgentPi, "session_shutdown", "execution.closed"},
		{agents.AgentPi, "session_compact", "session.compacted"},
	}
	for _, tc := range cases {
		t.Run(string(tc.agent)+"/"+tc.native, func(t *testing.T) {
			m, ok := mapping.EventMapFor(tc.agent)
			if !ok {
				t.Fatal("agent event map is missing")
			}
			for _, event := range m.Events {
				if event.NativeType == tc.native {
					if event.MnemoType != tc.normalized {
						t.Fatalf("mapping changed: %s -> %s, want %s", event.NativeType, event.MnemoType, tc.normalized)
					}
					return
				}
			}
			t.Fatalf("event %q is not mapped", tc.native)
		})
	}
}
