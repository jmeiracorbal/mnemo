package agentinit

import "testing"

func TestAgentSpecsDefineSupportedAgents(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range SupportedAgents {
		spec, ok := Spec(id)
		if !ok {
			t.Fatalf("missing spec for supported agent %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate supported agent %q", id)
		}
		seen[id] = true
		if spec.ID != id {
			t.Fatalf("spec ID = %q, want %q", spec.ID, id)
		}
		if spec.Label == "" {
			t.Fatalf("%s: missing label", id)
		}
		if spec.Detect == nil {
			t.Fatalf("%s: missing detection", id)
		}
		if spec.Supports.MCP && (spec.MCP.Snippets == nil || spec.MCP.Uninstall == nil || spec.MCP.Check == nil) {
			t.Fatalf("%s: MCP capability missing contract functions", id)
		}
		if spec.Supports.Instructions && len(spec.Instructions) == 0 {
			t.Fatalf("%s: instructions capability missing specs", id)
		}
	}
	if len(seen) != len(agentSpecs) {
		t.Fatalf("supported agents = %d, specs = %d", len(seen), len(agentSpecs))
	}
}

func TestAgentSpecCapabilitiesNameBelongsToAgentSpec(t *testing.T) {
	var _ AgentSpecCapabilities
}
