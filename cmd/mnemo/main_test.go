package main

import (
	"os"
	"testing"
)

// TestMain keeps tests from resolving the developer's real Claude Code
// profile: tests use temporary home directories, and the Claude environment
// would otherwise redirect Claude paths outside them.
func TestMain(m *testing.M) {
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	os.Unsetenv("CLAUDE_CODE_PROJECT_DIR_NAME")
	os.Exit(m.Run())
}
