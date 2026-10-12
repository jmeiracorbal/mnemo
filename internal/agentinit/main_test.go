package agentinit

import (
	"os"
	"testing"
)

// TestMain keeps tests from resolving the developer's real Claude Code
// profile: tests use temporary home directories, and the Claude environment
// (or managed settings) would otherwise redirect Claude paths outside them.
func TestMain(m *testing.M) {
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "CLAUDE_CODE_PROJECT_DIR_NAME"} {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	claudeManagedSettingsDir = ""
	os.Exit(m.Run())
}
