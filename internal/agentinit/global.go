package agentinit

import (
	"fmt"
)

// SupportedAgents is the canonical set of agent IDs accepted by mnemo init and
// the installer helpers. Keep this list stable for scripts and docs.
var SupportedAgents = agentSpecIDs(agentSpecs)

const (
	AgentClaudeCode = "claudecode"
	AgentCursor     = "cursor"
	AgentWindsurf   = "windsurf"
	AgentCodex      = "codex"
	AgentOpenCode   = "opencode"
	AgentFx         = "fx"
	AgentPi         = "pi"
)

// ExpandAgents resolves a single agent flag value to one or more concrete agent
// IDs. It accepts the special value "all".
func ExpandAgents(agent string) ([]string, error) {
	if agent == "all" {
		return append([]string(nil), SupportedAgents...), nil
	}
	if _, ok := Spec(agent); ok {
		return []string{agent}, nil
	}
	return nil, fmt.Errorf("unknown agent %q — valid: %s", agent, validAgentList())
}

// GlobalInstructionPath returns the user-scope instruction path used by an agent.
func GlobalInstructionPath(home, agent string) (string, error) {
	spec, err := lookupAgentSpec(agent)
	if err != nil {
		return "", err
	}
	instruction, ok := globalInstructionSpec(spec)
	if !ok || instruction.Path == nil {
		return "", fmt.Errorf("agent %q has no global instruction path", agent)
	}
	return instruction.Path(home), nil
}

// InstallGlobalInstructions writes the short, conditional mnemo instructions to
// an agent's global instruction surface. The block is conditional on a valid
// project .mnemo marker so a global install does not activate mnemo everywhere.
func InstallGlobalInstructions(home, agent string) (string, error) {
	spec, err := lookupAgentSpec(agent)
	if err != nil {
		return "", err
	}
	instruction, ok := globalInstructionSpec(spec)
	if !ok || instruction.Install == nil {
		return "", fmt.Errorf("agent %q has no global instruction installer", agent)
	}
	return instruction.Install(home)
}

// RemoveGlobalInstructions removes mnemo's global instruction surface for an
// agent while preserving user content outside the managed mnemo section.
func RemoveGlobalInstructions(home, agent string) (string, bool, error) {
	spec, err := lookupAgentSpec(agent)
	if err != nil {
		return "", false, err
	}
	instruction, ok := globalInstructionSpec(spec)
	if !ok || instruction.Remove == nil {
		return "", false, fmt.Errorf("agent %q has no global instruction remover", agent)
	}
	return instruction.Remove(home)
}
