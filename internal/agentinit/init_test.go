package agentinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallProjectInstructionsWritesExpectedFiles(t *testing.T) {
	root := t.TempDir()

	paths, err := InstallProjectInstructions(root, "cursor")
	if err != nil {
		t.Fatalf("install cursor project instructions: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 updated paths, got %d: %v", len(paths), paths)
	}

	agentsData, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsData), sectionStart) {
		t.Fatal("AGENTS.md missing managed mnemo section")
	}

	cursorData, err := os.ReadFile(filepath.Join(root, ".cursor", "rules", "mnemo.mdc"))
	if err != nil {
		t.Fatalf("read cursor rule: %v", err)
	}
	if !strings.Contains(string(cursorData), "MEMORY AUTHORITY") {
		t.Fatal("cursor rule missing memory authority")
	}
}

func TestInstallProjectInstructionsIsIdempotent(t *testing.T) {
	root := t.TempDir()

	for i := 0; i < 2; i++ {
		if _, err := InstallProjectInstructions(root, "codex"); err != nil {
			t.Fatalf("install attempt %d: %v", i+1, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if strings.Count(string(data), sectionStart) != 1 {
		t.Fatalf("expected one managed section, got %d", strings.Count(string(data), sectionStart))
	}
}

func TestInstallProjectInstructionsClaudeCode(t *testing.T) {
	root := t.TempDir()

	paths, err := InstallProjectInstructions(root, "claudecode")
	if err != nil {
		t.Fatalf("install claudecode project instructions: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}

	claudeData, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeData), claudeSectionStart) {
		t.Fatal("CLAUDE.md missing claude managed section")
	}
	if !strings.Contains(string(claudeData), "@AGENTS.md") {
		t.Fatal("CLAUDE.md missing @AGENTS.md reference")
	}
}

func TestInstallProjectInstructionsWindsurf(t *testing.T) {
	root := t.TempDir()

	paths, err := InstallProjectInstructions(root, "windsurf")
	if err != nil {
		t.Fatalf("install windsurf project instructions: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}

	windsurfPath := filepath.Join(root, ".windsurf", "rules", "mnemo.md")
	if _, err := os.Stat(windsurfPath); err != nil {
		t.Fatalf("windsurf rule missing: %v", err)
	}
}

func TestInstallProjectInstructionsPi(t *testing.T) {
	root := t.TempDir()

	paths, err := InstallProjectInstructions(root, "pi")
	if err != nil {
		t.Fatalf("install pi project instructions: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}

	piPath := filepath.Join(root, ".pi", "APPEND_SYSTEM.md")
	piData, err := os.ReadFile(piPath)
	if err != nil {
		t.Fatalf("read Pi APPEND_SYSTEM.md: %v", err)
	}
	if !strings.Contains(string(piData), "PI MEMORY GUIDANCE") {
		t.Fatalf("Pi project prompt extension missing memory guidance:\n%s", string(piData))
	}
}
