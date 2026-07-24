package agentinit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

// SupportedAgents is the canonical set of agent IDs accepted by mnemo init and
// the installer helpers. Keep this list stable for scripts and docs.
var SupportedAgents = []string{"claudecode", "cursor", "windsurf", "codex", "opencode"}

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

// GlobalInstructionPath returns the user-scope instruction path used by an
// agent, mirroring CodeGraph's global install model.
func GlobalInstructionPath(home, agent string) (string, error) {
	switch agent {
	case "claudecode":
		return filepath.Join(home, ".claude", "CLAUDE.md"), nil
	case "cursor":
		return filepath.Join(home, ".cursor", "rules", "mnemo.mdc"), nil
	case "windsurf":
		return filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md"), nil
	case "codex":
		return filepath.Join(home, ".codex", "AGENTS.md"), nil
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "AGENTS.md"), nil
	default:
		return "", fmt.Errorf("unknown agent %q", agent)
	}
}

// InstallGlobalInstructions writes the short, conditional mnemo instructions to
// an agent's global instruction surface. The block is conditional on a valid
// project .mnemo marker so a global install does not activate mnemo everywhere.
func InstallGlobalInstructions(home, agent string) (string, error) {
	path, err := GlobalInstructionPath(home, agent)
	if err != nil {
		return "", err
	}
	if agent == "cursor" {
		if err := WriteFile(path, []byte(templates.CursorGlobal)); err != nil {
			return "", err
		}
		return path, nil
	}
	if err := AppendSection(path, templates.Global); err != nil {
		return "", err
	}
	return path, nil
}

// RemoveGlobalInstructions removes mnemo's global instruction surface for an
// agent while preserving user content outside the managed mnemo section.
func RemoveGlobalInstructions(home, agent string) (string, bool, error) {
	path, err := GlobalInstructionPath(home, agent)
	if err != nil {
		return "", false, err
	}
	if agent == "cursor" {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return path, false, nil
			}
			return path, false, err
		}
		return path, true, nil
	}
	changed, err := RemoveSection(path)
	return path, changed, err
}
