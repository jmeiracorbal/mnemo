package agentinit

import (
	"os"
	"testing"
)

// TestMain keeps tests from resolving the developer's real Claude Code
// profile: tests use temporary home directories, and the Claude environment
// (or managed settings) would otherwise redirect Claude paths outside them.
func TestMain(m *testing.M) {
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	os.Unsetenv("CLAUDE_CODE_PROJECT_DIR_NAME")
	claudeManagedSettingsDir = ""
	os.Exit(m.Run())
}
