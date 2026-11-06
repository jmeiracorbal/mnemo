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

func TestProjectIDForMigrationPreservesExistingMarker(t *testing.T) {
	root := t.TempDir()
	marker := "{\n  \"version\": 1,\n  \"id\": \"existing-project-id\",\n  \"agents\": [\"cursor\"]\n}\n"
	path := filepath.Join(root, ".mnemo")
	if err := os.WriteFile(path, []byte(marker), 0644); err != nil {
		t.Fatal(err)
	}
	id, err := ProjectIDForMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if id != "existing-project-id" {
		t.Fatalf("migration changed project ID: %q", id)
	}
	if err := EnsureMarkerWithID(root, id); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != marker {
		t.Fatalf("existing marker was rewritten: %q", got)
	}
}

func TestProjectIDForMigrationDerivesIDOnlyForLegacyMarkers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker string
	}{
		{name: "missing marker"},
		{name: "marker without ID", marker: "{\"version\":1,\"agents\":[\"codex\"]}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".mnemo")
			if tc.marker != "" {
				if err := os.WriteFile(path, []byte(tc.marker), 0644); err != nil {
					t.Fatal(err)
				}
			}
			id, err := ProjectIDForMigration(root)
			if err != nil {
				t.Fatal(err)
			}
			if expected := ProjectUUIDFromPath(root); id != expected {
				t.Fatalf("wrong derived ID: got %s want %s", id, expected)
			}
			got, err := os.ReadFile(path)
			if tc.marker == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("migration ID lookup created marker: %v", err)
				}
			} else if err != nil || string(got) != tc.marker {
				t.Fatalf("migration ID lookup mutated legacy marker: %q, %v", got, err)
			}
		})
	}
}

func TestProjectIDForMigrationRejectsMalformedMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mnemo"), []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProjectIDForMigration(root); err == nil {
		t.Fatal("expected malformed marker error")
	}
}
