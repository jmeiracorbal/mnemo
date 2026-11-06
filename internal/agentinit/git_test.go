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

func writeMarkerForTest(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".mnemo"), []byte(`{"version":1,"id":"stable-uuid","agents":["codex"]}`), 0644); err != nil {
		t.Fatal(err)
	}
}

func gitRepoForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if out, err := gitRun(t, root, "init", "-q"); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	return root
}

func TestEnsureMarkerTrackedOutsideGitDoesNotCreateGitignore(t *testing.T) {
	root := t.TempDir()
	writeMarkerForTest(t, root)
	if err := EnsureMarkerTracked(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("unexpected .gitignore: %v", err)
	}
}

func TestEnsureMarkerTrackedRemovesLegacyIgnoreAndStagesMarker(t *testing.T) {
	for _, ignore := range []string{".mnemo", "/.mnemo"} {
		t.Run(ignore, func(t *testing.T) {
			root := gitRepoForTest(t)
			writeMarkerForTest(t, root)
			original := "# project\n*.log\n" + ignore + "\n.keep\n"
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := EnsureMarkerTracked(root); err != nil {
					t.Fatalf("attempt %d: %v", attempt, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(data), "# project\n*.log\n.keep\n"; got != want {
				t.Fatalf("unexpected ignore file: %q want %q", got, want)
			}
			if out, err := gitRun(t, root, "ls-files", "--error-unmatch", "--", ".mnemo"); err != nil {
				t.Fatalf("marker not tracked: %s %v", out, err)
			}
			if out, err := gitRun(t, root, "diff", "--cached", "--name-only"); err != nil || strings.TrimSpace(out) != ".mnemo" {
				t.Fatalf("unexpected staging: %q %v", out, err)
			}
		})
	}
}

func TestEnsureMarkerTrackedStagesNewFileWithoutCreatingGitignore(t *testing.T) {
	root := gitRepoForTest(t)
	writeMarkerForTest(t, root)
	if err := EnsureMarkerTracked(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("unexpected .gitignore: %v", err)
	}
	if out, err := gitRun(t, root, "ls-files", "--error-unmatch", "--", ".mnemo"); err != nil {
		t.Fatalf("marker not tracked: %s %v", out, err)
	}
}

func TestEnsureMarkerTrackedDoesNotRestageAnExistingMarker(t *testing.T) {
	root := gitRepoForTest(t)
	writeMarkerForTest(t, root)
	if out, err := gitRun(t, root, "add", "--", ".mnemo"); err != nil {
		t.Fatalf("initial add: %s %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mnemo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mnemo"), []byte(`{"version":1,"id":"stable-uuid","agents":["cursor"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureMarkerTracked(root); err != nil {
		t.Fatal(err)
	}
	if out, err := gitRun(t, root, "diff", "--", ".mnemo"); err != nil || !strings.Contains(out, "cursor") {
		t.Fatalf("existing marker was unexpectedly restaged: %s %v", out, err)
	}
}

func TestEnsureMarkerTrackedRejectsOtherIgnoreSources(t *testing.T) {
	root := gitRepoForTest(t)
	writeMarkerForTest(t, root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mnemo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".mnemo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := EnsureMarkerTracked(root)
	if err == nil || !strings.Contains(err.Error(), "still excluded") {
		t.Fatalf("expected other ignore source error, got %v", err)
	}
	if out, err := gitRun(t, root, "ls-files", "--error-unmatch", "--", ".mnemo"); err == nil {
		t.Fatalf("marker should not have been staged: %s", out)
	}
}

func TestRemoveMarkerIgnoreRulePreservesUnrelatedPatterns(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	original := "other/.mnemo\r\n!.mnemo\r\n# .mnemo\r\n/.mnemo\r\n*.log"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeMarkerIgnoreRule(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "other/.mnemo\r\n!.mnemo\r\n# .mnemo\r\n*.log"; got != want {
		t.Fatalf("unexpected content: %q want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected permissions: %s", info.Mode())
	}
}

func TestEnsureMarkerTrackedHonorsNegatedIgnoreRule(t *testing.T) {
	root := gitRepoForTest(t)
	writeMarkerForTest(t, root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("!.mnemo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureMarkerTracked(root); err != nil {
		t.Fatal(err)
	}
	if out, err := gitRun(t, root, "ls-files", "--error-unmatch", "--", ".mnemo"); err != nil {
		t.Fatalf("marker not tracked: %s %v", out, err)
	}
}
