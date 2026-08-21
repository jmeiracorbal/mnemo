package agentinit

import (
	"fmt"
)

// SupportedAgents is the canonical set of agent IDs accepted by mnemo init and
// the installer helpers. Keep this list stable for scripts and docs.
var SupportedAgents = []string{"claudecode", "cursor", "windsurf", "codex", "opencode"}

const (
	AgentClaudeCode = "claudecode"
	AgentCursor     = "cursor"
	AgentWindsurf   = "windsurf"
	AgentCodex      = "codex"
	AgentOpenCode   = "opencode"
)

// ExpandAgents resolves a single agent flag value to one or more concrete agent
// IDs. It accepts the special value "all".
func ExpandAgents(agent string) ([]string, error) {
	if agent == "all" {
		return append([]string(nil), SupportedAgents...), nil
	}
	for _, a := range SupportedAgents {
		if agent == a {
			return []string{agent}, nil
		}
	}
	return nil, fmt.Errorf("unknown agent %q — valid: claudecode | cursor | windsurf | codex | opencode | all", agent)
}

// GlobalInstructionPath returns the user-scope instruction path used by an agent.
func GlobalInstructionPath(home, agent string) (string, error) {
	switch agent {
	case "claudecode":
		return claudeCodeInstructionPath(home), nil
	case "cursor":
		return cursorInstructionPath(home), nil
	case "windsurf":
		return windsurfInstructionPath(home), nil
	case "codex":
		return codexInstructionPath(home), nil
	case "opencode":
		return openCodeInstructionPath(home), nil
	default:
		return "", fmt.Errorf("unknown agent %q", agent)
	}
}

// InstallGlobalInstructions writes the short, conditional mnemo instructions to
// an agent's global instruction surface. The block is conditional on a valid
// project .mnemo marker so a global install does not activate mnemo everywhere.
func InstallGlobalInstructions(home, agent string) (string, error) {
	switch agent {
	case "claudecode":
		return claudeCodeInstallInstructions(home)
	case "cursor":
		return cursorInstallInstructions(home)
	case "windsurf":
		return windsurfInstallInstructions(home)
	case "codex":
		return codexInstallInstructions(home)
	case "opencode":
		return openCodeInstallInstructions(home)
	default:
		return "", fmt.Errorf("unknown agent %q", agent)
	}
}

// RemoveGlobalInstructions removes mnemo's global instruction surface for an
// agent while preserving user content outside the managed mnemo section.
func RemoveGlobalInstructions(home, agent string) (string, bool, error) {
	switch agent {
	case "claudecode":
		return claudeCodeRemoveInstructions(home)
	case "cursor":
		return cursorRemoveInstructions(home)
	case "windsurf":
		return windsurfRemoveInstructions(home)
	case "codex":
		return codexRemoveInstructions(home)
	case "opencode":
		return openCodeRemoveInstructions(home)
	default:
		return "", false, fmt.Errorf("unknown agent %q", agent)
	}
}
