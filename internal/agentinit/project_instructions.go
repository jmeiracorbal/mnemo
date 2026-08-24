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

	spec, ok := Spec(agent)
	if !ok {
		return nil, fmt.Errorf("unsupported agent %q for project instructions", agent)
	}
	for _, instruction := range projectInstructionSpecs(spec) {
		if instruction.Install == nil {
			return nil, fmt.Errorf("agent %q has an incomplete project instruction spec", agent)
		}
		path, err := instruction.Install(root)
		if err != nil {
			return nil, err
		}
		updated = append(updated, path)
	}

	return updated, nil
}

func claudeCodeProjectInstructionPath(root string) string {
	return filepath.Join(root, "CLAUDE.md")
}

func claudeCodeInstallProjectInstructions(root string) (string, error) {
	path := claudeCodeProjectInstructionPath(root)
	if err := AppendClaudeSection(path, templates.ClaudeCode); err != nil {
		return "", fmt.Errorf("CLAUDE.md: %w", err)
	}
	return path, nil
}

func cursorProjectInstructionPath(root string) string {
	return filepath.Join(root, ".cursor", "rules", "mnemo.mdc")
}

func cursorInstallProjectInstructions(root string) (string, error) {
	path := cursorProjectInstructionPath(root)
	if err := WriteFile(path, []byte(templates.Cursor)); err != nil {
		return "", fmt.Errorf(".cursor/rules/mnemo.mdc: %w", err)
	}
	return path, nil
}

func windsurfProjectInstructionPath(root string) string {
	return filepath.Join(root, ".windsurf", "rules", "mnemo.md")
}

func windsurfInstallProjectInstructions(root string) (string, error) {
	path := windsurfProjectInstructionPath(root)
	if err := AppendSection(path, templates.Windsurf); err != nil {
		return "", fmt.Errorf(".windsurf/rules/mnemo.md: %w", err)
	}
	return path, nil
}
