package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runTrackGit(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestTrackProjectStagesExistingMarkerWithoutChangingIdentity(t *testing.T) {
	root := t.TempDir()
	if out, err := runTrackGit(t, root, "init", "-q"); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	original := `{"version":1,"id":"stable-project-id","agents":["codex"]}` + "\n"
	if err := os.WriteFile(filepath.Join(root, ".mnemo"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mnemo\n.env\n"), 0644); err != nil {
		t.Fatal(err)
	}

	expectedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		gotRoot, id, err := trackProject(root)
		if err != nil {
			t.Fatalf("track attempt %d: %v", attempt, err)
		}
		if gotRoot != expectedRoot || id != "stable-project-id" {
			t.Fatalf("unexpected identity: root %q (expected %q), id %q", gotRoot, expectedRoot, id)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".mnemo"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("track changed .mnemo content: %q", data)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ignore) != ".env\n" {
		t.Fatalf("track removed unrelated ignore rule: %q", ignore)
	}
	if out, err := runTrackGit(t, root, "diff", "--cached", "--name-only"); err != nil || strings.TrimSpace(out) != ".mnemo" {
		t.Fatalf("only marker should be staged: %q, error: %v", out, err)
	}
}

func TestTrackProjectRequiresInitializedMarker(t *testing.T) {
	root := t.TempDir()
	if out, err := runTrackGit(t, root, "init", "-q"); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	if _, _, err := trackProject(root); err == nil || !strings.Contains(err.Error(), ".mnemo") {
		t.Fatalf("expected missing marker error, got %v", err)
	}
}

func TestTrackProjectRequiresGitRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mnemo"), []byte(`{"id":"stable-project-id"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := trackProject(root); err == nil || !strings.Contains(err.Error(), "not a Git repository") {
		t.Fatalf("expected non-Git error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("track unexpectedly initialized Git: %v", err)
	}
}

func TestTrackCommandIsRegistered(t *testing.T) {
	command, _, err := newRootCommand().Find([]string{"track"})
	if err != nil {
		t.Fatalf("find track command: %v", err)
	}
	if command == nil || command.Name() != "track" {
		t.Fatalf("unexpected command: %v", command)
	}
}
