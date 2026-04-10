package jsonmerge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runMerge is a helper that sets stdin to the given patch JSON and calls MergeFile.
func runMerge(t *testing.T, filePath, patchJSON string) (bool, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString(patchJSON); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	return MergeFile(filePath)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

func TestMergeFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	patch := `{"mcpServers":{"mnemo":{"command":"/usr/local/bin/mnemo","args":["mcp"]}}}`
	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for new file")
	}

	got := readJSON(t, path)
	servers, ok := got["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers not a map: %T", got["mcpServers"])
	}
	if _, exists := servers["mnemo"]; !exists {
		t.Error("expected mcpServers.mnemo to exist")
	}
}

func TestMergeFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	patch := `{"mcpServers":{"mnemo":{"command":"/usr/local/bin/mnemo","args":["mcp"]}}}`

	// First write.
	if _, err := runMerge(t, path, patch); err != nil {
		t.Fatalf("first merge: %v", err)
	}

	// Second write: same patch — should be no-op.
	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if changed {
		t.Error("expected changed=false for idempotent merge")
	}
}

func TestMergeFile_ArrayDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	initial := `{"hooks":{"stop":[{"command":"/existing/hook.sh"}]}}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	// Add a new hook.
	patch := `{"hooks":{"stop":[{"command":"/new/hook.sh"}]}}`
	if _, err := runMerge(t, path, patch); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	hooks := got["hooks"].(map[string]any)
	stop := hooks["stop"].([]any)
	if len(stop) != 2 {
		t.Errorf("expected 2 stop hooks, got %d", len(stop))
	}

	// Merge the same new hook again — should not duplicate.
	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if changed {
		t.Error("expected no-op when hook already present")
	}

	got = readJSON(t, path)
	hooks = got["hooks"].(map[string]any)
	stop = hooks["stop"].([]any)
	if len(stop) != 2 {
		t.Errorf("expected still 2 stop hooks after dedup, got %d", len(stop))
	}
}

func TestMergeFile_DeepObjectMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	initial := `{"mcpServers":{"other":{"command":"something"}}}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	patch := `{"mcpServers":{"mnemo":{"command":"/usr/local/bin/mnemo"}}}`
	if _, err := runMerge(t, path, patch); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	servers := got["mcpServers"].(map[string]any)
	if _, exists := servers["other"]; !exists {
		t.Error("existing key 'other' should be preserved")
	}
	if _, exists := servers["mnemo"]; !exists {
		t.Error("new key 'mnemo' should be added")
	}
}

func TestMergeFile_InvalidStdin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	_, err := runMerge(t, path, `not valid json`)
	if err == nil {
		t.Error("expected error for invalid stdin JSON")
	}
	if !strings.Contains(err.Error(), "read patch from stdin") {
		t.Errorf("unexpected error message: %v", err)
	}

	// File should not have been created.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should not be created on stdin error")
	}
}

func TestMergeFile_ParentDirCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "config.json")

	patch := `{"key":"value"}`
	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestDeepMerge_ScalarPatchWins(t *testing.T) {
	target := map[string]any{"version": float64(1)}
	patch := map[string]any{"version": float64(2)}
	result := deepMerge(target, patch).(map[string]any)
	if result["version"] != float64(2) {
		t.Errorf("expected patch value 2, got %v", result["version"])
	}
}

func TestDeepMerge_TypeMismatchPatchWins(t *testing.T) {
	// target has array, patch has object at same key: patch wins
	target := map[string]any{"hooks": []any{"a", "b"}}
	patch := map[string]any{"hooks": map[string]any{"stop": []any{}}}
	result := deepMerge(target, patch).(map[string]any)
	if _, ok := result["hooks"].(map[string]any); !ok {
		t.Errorf("expected hooks to be a map after type mismatch, got %T", result["hooks"])
	}
}
