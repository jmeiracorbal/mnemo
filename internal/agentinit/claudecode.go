package agentinit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

// InitClaudeCode configures mnemo for Claude Code in the given project root.
//
// Appends the generic mnemo protocol to AGENTS.md (shared with other agents) and
// creates CLAUDE.md as a real file containing an @AGENTS.md include plus the
// Claude Code-specific protocol additions. If CLAUDE.md already exists as a
// regular file, the Claude section is appended to it. If it was previously a
// symlink (old architecture), the symlink is removed first.
func InitClaudeCode(root string) error {
	agentsPath := filepath.Join(root, "AGENTS.md")
	claudePath := filepath.Join(root, "CLAUDE.md")

	if err := AppendSection(agentsPath, templates.Generic); err != nil {
		return fmt.Errorf("AGENTS.md: %w", err)
	}

	info, err := os.Lstat(claudePath)
	if err == nil {
		mode := info.Mode()
		if mode.IsDir() {
			return fmt.Errorf("CLAUDE.md: is a directory")
		}
		if mode&os.ModeSymlink != 0 {
			if err := os.Remove(claudePath); err != nil {
				return fmt.Errorf("CLAUDE.md: remove symlink: %w", err)
			}
		} else if !mode.IsRegular() {
			return fmt.Errorf("CLAUDE.md: unexpected file type %v", mode.Type())
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("CLAUDE.md: stat: %w", err)
	}

	if err := AppendClaudeSection(claudePath, templates.ClaudeCode); err != nil {
		return fmt.Errorf("CLAUDE.md: %w", err)
	}

	return AddAgent(root, "claudecode")
}
