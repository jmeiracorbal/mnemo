package agentinit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStopHooksDoNotEmitZeroMemoryWarning(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test filename")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	for _, path := range []string{
		"scripts/codex/hooks/stop.sh",
		"scripts/cursor/hooks/stop.sh",
		"scripts/windsurf/hooks/post-cascade-response.sh",
		"plugin/claude-code/scripts/session-stop.sh",
	} {
		t.Run(path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, path))
			if err != nil {
				t.Fatalf("read hook: %v", err)
			}
			if strings.Contains(string(content), "session ended with 0 memories saved") ||
				strings.Contains(string(content), "project-obs-count") {
				t.Fatalf("%s should end sessions silently when no memories were captured", path)
			}
		})
	}
}
