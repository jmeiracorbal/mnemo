package agentinit

import (
	"fmt"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

// InstallProjectInstructions writes agent-specific memory authority rules into
// the project repository. Idempotent via managed sections or dedicated paths.
func InstallProjectInstructions(root, agent string) ([]string, error) {
	var updated []string

	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := AppendSection(agentsPath, templates.Generic); err != nil {
		return nil, fmt.Errorf("AGENTS.md: %w", err)
	}
	updated = append(updated, agentsPath)

	switch agent {
	case "claudecode":
		claudePath := filepath.Join(root, "CLAUDE.md")
		if err := AppendClaudeSection(claudePath, templates.ClaudeCode); err != nil {
			return nil, fmt.Errorf("CLAUDE.md: %w", err)
		}
		updated = append(updated, claudePath)
	case "cursor":
		cursorPath := filepath.Join(root, ".cursor", "rules", "mnemo.mdc")
		if err := WriteFile(cursorPath, []byte(templates.Cursor)); err != nil {
			return nil, fmt.Errorf(".cursor/rules/mnemo.mdc: %w", err)
		}
		updated = append(updated, cursorPath)
	case "windsurf":
		windsurfPath := filepath.Join(root, ".windsurf", "rules", "mnemo.md")
		if err := AppendSection(windsurfPath, templates.Windsurf); err != nil {
			return nil, fmt.Errorf(".windsurf/rules/mnemo.md: %w", err)
		}
		updated = append(updated, windsurfPath)
	case "codex", "opencode":
		// AGENTS.md covers codex and opencode project authority.
	default:
		return nil, fmt.Errorf("unsupported agent %q for project instructions", agent)
	}

	return updated, nil
}
