package agentinit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

// InitOpenCode configures mnemo for OpenCode in the given project root.
//
// Appends the mnemo protocol section to AGENTS.md. Hooks for OpenCode are
// registered globally by install.sh as a TypeScript plugin; they check for
// the .mnemo marker at runtime.
func InitOpenCode(root string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("opencode init: could not determine HOME: %w", err)
	}

	pluginPath := filepath.Join(home, ".config", "opencode", "plugins", "mnemo.ts")
	if _, err := os.Stat(pluginPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("opencode init: plugin not found: %s\nRun install.sh --agent=opencode first", pluginPath)
		}
		return fmt.Errorf("opencode init: stat %s: %w", pluginPath, err)
	}

	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := AppendSection(agentsPath, templates.Generic); err != nil {
		return fmt.Errorf("opencode init: AGENTS.md: %w", err)
	}
	return AddAgent(root, "opencode")
}
