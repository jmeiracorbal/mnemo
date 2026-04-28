package agentinit

import (
	_ "embed"
	"fmt"
	"path/filepath"
)

//go:embed templates/codex.md
var codexProtocol string

// InitCodex configures mnemo for Codex in the given project root.
//
// Appends the mnemo protocol section to AGENTS.md. Hooks for Codex are global
// (registered by install.sh); they check for the .mnemo marker at runtime.
func InitCodex(root string) error {
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := AppendSection(agentsPath, codexProtocol); err != nil {
		return fmt.Errorf("codex init: AGENTS.md: %w", err)
	}
	return AddAgent(root, "codex")
}
