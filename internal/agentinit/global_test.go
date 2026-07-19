package agentinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallGlobalInstructionsWritesConditionalAgentFiles(t *testing.T) {
	home := t.TempDir()

	cases := map[string]string{
		"claudecode": filepath.Join(home, ".claude", "CLAUDE.md"),
		"cursor":     filepath.Join(home, ".cursor", "rules", "mnemo.mdc"),
		"windsurf":   filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md"),
		"codex":      filepath.Join(home, ".codex", "AGENTS.md"),
		"opencode":   filepath.Join(home, ".config", "opencode", "AGENTS.md"),
	}

	for agent, wantPath := range cases {
		t.Run(agent, func(t *testing.T) {
			gotPath, err := InstallGlobalInstructions(home, agent)
			if err != nil {
				t.Fatalf("install global instructions: %v", err)
			}
			if gotPath != wantPath {
				t.Fatalf("path = %q, want %q", gotPath, wantPath)
			}

			data, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("read global instructions: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, ".mnemo") {
				t.Fatalf("global instructions are not conditional on .mnemo:\n%s", content)
			}
			if !strings.Contains(content, "ONLY persistent memory system") {
				t.Fatalf("global instructions do not declare memory authority:\n%s", content)
			}
			if !strings.Contains(content, "skip mnemo entirely") {
				t.Fatalf("global instructions do not tell agents to skip uninitialized projects:\n%s", content)
			}
		})
	}
}

func TestInstallGlobalInstructionsPreservesExistingMarkedFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Existing\n\nUser content.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallGlobalInstructions(home, "codex"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := InstallGlobalInstructions(home, "codex"); err != nil {
		t.Fatalf("second install: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# Existing\n\nUser content.\n") {
		t.Fatalf("existing content was not preserved:\n%s", content)
	}
	if strings.Count(content, sectionStart) != 1 || strings.Count(content, sectionEnd) != 1 {
		t.Fatalf("managed section not idempotent:\n%s", content)
	}
}
