package agentinit

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/claudecode.md
var claudeProtocol string

// InitClaudeCode configures mnemo for Claude Code in the given project root.
//
// Creates AGENTS.md with the mnemo protocol section and makes CLAUDE.md a
// symlink to AGENTS.md. If CLAUDE.md already exists as a regular file, its
// content is moved to AGENTS.md before the symlink is created.
func InitClaudeCode(root string) error {
	agentsPath := filepath.Join(root, "AGENTS.md")
	claudePath := filepath.Join(root, "CLAUDE.md")

	if err := AppendSection(agentsPath, claudeProtocol); err != nil {
		return fmt.Errorf("AGENTS.md: %w", err)
	}

	info, err := os.Lstat(claudePath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, linkErr := os.Readlink(claudePath)
			if linkErr == nil && (target == "AGENTS.md" || target == agentsPath) {
				return AddAgent(root, "claudecode")
			}
			if err := os.Remove(claudePath); err != nil {
				return fmt.Errorf("CLAUDE.md: remove stale symlink: %w", err)
			}
		} else {
			existing, err := os.ReadFile(claudePath)
			if err != nil {
				return fmt.Errorf("CLAUDE.md: read: %w", err)
			}
			if len(existing) > 0 {
				if err := prependToFile(agentsPath, string(existing)); err != nil {
					return fmt.Errorf("AGENTS.md: prepend CLAUDE.md content: %w", err)
				}
			}
			if err := os.Remove(claudePath); err != nil {
				return fmt.Errorf("CLAUDE.md: remove: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("CLAUDE.md: stat: %w", err)
	}

	if err := os.Symlink("AGENTS.md", claudePath); err != nil {
		return fmt.Errorf("CLAUDE.md: create symlink: %w", err)
	}

	return AddAgent(root, "claudecode")
}
