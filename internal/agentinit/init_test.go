package agentinit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectInitDoesNotWriteProjectInstructionFiles(t *testing.T) {
	root := t.TempDir()
	for _, initFn := range []func(string) error{InitClaudeCode, InitCursor, InitWindsurf, InitCodex, InitOpenCode} {
		if err := initFn(root); err != nil {
			t.Fatalf("init agent: %v", err)
		}
	}

	for _, rel := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		filepath.Join(".cursor", "hooks.json"),
		filepath.Join(".cursor", "rules", "mnemo.mdc"),
		filepath.Join(".windsurf", "hooks.json"),
		filepath.Join(".windsurf", "rules", "mnemo.md"),
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("project init unexpectedly wrote %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
}
