package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEventValidateMarkerUsesGitRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	const rootID = "aaaaaaaa-0000-0000-0000-000000000001"
	const subID = "bbbbbbbb-0000-0000-0000-000000000002"

	writeFile(t, filepath.Join(root, ".mnemo"), `{"id":"`+rootID+`"}`)

	sub := filepath.Join(root, "pkg", "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	// A rogue .mnemo in a subdirectory — must not satisfy the guardrail.
	writeFile(t, filepath.Join(sub, ".mnemo"), `{"id":"`+subID+`"}`)

	t.Run("subdirectory id bypasses guardrail", func(t *testing.T) {
		err := eventValidateMarker(subID, sub)
		if err == nil {
			t.Fatal("expected error: subdirectory .mnemo id should not satisfy guardrail")
		}
	})

	t.Run("root id accepted from subdirectory", func(t *testing.T) {
		if err := eventValidateMarker(rootID, sub); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("root id accepted from root", func(t *testing.T) {
		if err := eventValidateMarker(rootID, root); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("wrong id rejected", func(t *testing.T) {
		err := eventValidateMarker("cccccccc-0000-0000-0000-000000000003", root)
		if err == nil {
			t.Fatal("expected error: id mismatch should be rejected")
		}
	})

	t.Run("missing marker rejected", func(t *testing.T) {
		empty := t.TempDir()
		if out, err := exec.Command("git", "-C", empty, "init").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		if err := eventValidateMarker(rootID, empty); err == nil {
			t.Fatal("expected error: no .mnemo should fail")
		}
	})
}
