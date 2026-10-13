package main

import (
	"os"
	"testing"
)

// TestMain keeps tests from resolving the developer's real Claude Code
// profile: tests use temporary home directories, and the Claude environment
// would otherwise redirect Claude paths outside them.
//
// With MNEMO_TEST_RUN_MAIN set, the test binary runs mnemo's main instead, so
// tests can put it on PATH as the mnemo command that shipped hooks invoke.
func TestMain(m *testing.M) {
	if os.Getenv("MNEMO_TEST_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "CLAUDE_CODE_PROJECT_DIR_NAME"} {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
