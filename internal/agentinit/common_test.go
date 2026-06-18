package agentinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendSectionCreatesAndUpdatesManagedBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("# Project\n\nUser content.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := AppendSection(path, "first protocol"); err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := AppendSection(path, "updated protocol"); err != nil {
		t.Fatalf("update section: %v", err)
	}
	if err := AppendSection(path, "updated protocol"); err != nil {
		t.Fatalf("idempotent update: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "# Project\n\nUser content.\n") {
		t.Fatalf("user content was not preserved:\n%s", got)
	}
	if strings.Contains(got, "first protocol") {
		t.Fatalf("old managed content remains:\n%s", got)
	}
	if strings.Count(got, sectionStart) != 1 || strings.Count(got, sectionEnd) != 1 {
		t.Fatalf("managed markers duplicated:\n%s", got)
	}
	if strings.Count(got, "updated protocol") != 1 {
		t.Fatalf("updated content missing or duplicated:\n%s", got)
	}
}

func TestAppendSectionRejectsMalformedMarkersWithoutChangingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	original := "before\n" + sectionStart + "\nmissing end\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	err := AppendSection(path, "replacement")
	if err == nil || !strings.Contains(err.Error(), "malformed managed section") {
		t.Fatalf("expected malformed marker error, got %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != original {
		t.Fatalf("file changed after malformed marker error:\n%s", data)
	}
}

func TestAppendClaudeSectionAddsIncludeOnceAndUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := AppendClaudeSection(path, "first"); err != nil {
		t.Fatalf("create Claude section: %v", err)
	}
	if err := AppendClaudeSection(path, "second"); err != nil {
		t.Fatalf("update Claude section: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Count(got, "@AGENTS.md") != 1 {
		t.Fatalf("@AGENTS.md include duplicated or missing:\n%s", got)
	}
	if strings.Contains(got, "first") || strings.Count(got, "second") != 1 {
		t.Fatalf("Claude section was not replaced:\n%s", got)
	}
}

func TestGlobalSkillInstalled(t *testing.T) {
	home := t.TempDir()
	if GlobalSkillInstalled(home) {
		t.Fatal("skill reported installed before file exists")
	}

	path := GlobalSkillPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: mnemo-memory\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !GlobalSkillInstalled(home) {
		t.Fatal("skill not detected at canonical global path")
	}
}
