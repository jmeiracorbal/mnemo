package agentinit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRun(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitRepoForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if out, err := gitRun(t, root, "init", "-q"); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	return root
}

func TestEnsureMarkerIgnoredIsIdempotentAndPreservesOtherRules(t *testing.T) {
	root := gitRepoForTest(t)
	original := "# keep\n.env\n!.mnemo\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := EnsureMarkerIgnored(root); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	want := original + "/.mnemo\n"
	if string(got) != want {
		t.Fatalf("unexpected ignore contents: %q want %q", got, want)
	}
	info, err := os.Stat(filepath.Join(root, ".gitignore"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions changed: %v %v", info, err)
	}
	if out, err := gitRun(t, root, "check-ignore", "-q", "--", ".mnemo"); err != nil {
		t.Fatalf("marker is not excluded: %s %v", out, err)
	}
	if out, err := gitRun(t, root, "ls-files", "--", ".mnemo"); err != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("marker unexpectedly staged: %s %v", out, err)
	}
}

func TestEnsureMarkerIgnoredCreatesRuleOnlyInGit(t *testing.T) {
	root := gitRepoForTest(t)
	if err := EnsureMarkerIgnored(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || string(got) != "/.mnemo\n" {
		t.Fatalf("unexpected ignore file: %q %v", got, err)
	}
	withoutGit := t.TempDir()
	if err := EnsureMarkerIgnored(withoutGit); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(withoutGit, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("non-Git directory received ignore file: %v", err)
	}
}

func TestEnsureMarkerIgnoredRejectsAlreadyTrackedMarkerWithoutChangingFiles(t *testing.T) {
	root := gitRepoForTest(t)
	path := filepath.Join(root, ".mnemo")
	if err := os.WriteFile(path, []byte("existing marker\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := gitRun(t, root, "add", "--", ".mnemo"); err != nil {
		t.Fatalf("git add: %s %v", out, err)
	}
	err := EnsureMarkerIgnored(root)
	if err == nil || !strings.Contains(err.Error(), "git rm --cached") {
		t.Fatalf("tracked marker must fail explicitly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("unexpected .gitignore modification: %v", err)
	}
	if out, err := gitRun(t, root, "ls-files", "--error-unmatch", "--", ".mnemo"); err != nil {
		t.Fatalf("tracked marker was removed from index: %s %v", out, err)
	}
}
