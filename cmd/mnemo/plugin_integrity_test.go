package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShippedHooksReferenceRealScripts validates that every command in
// plugin/claude-code/hooks/hooks.json points to a script that actually
// exists in plugin/claude-code/scripts/. This catches filename mismatches
// before they reach a published release.
func TestShippedHooksReferenceRealScripts(t *testing.T) {
	hooksFile := filepath.Join("..", "..", "plugin", "claude-code", "hooks", "hooks.json")
	data, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("could not read shipped hooks.json: %v", err)
	}

	var raw struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("could not parse hooks.json: %v", err)
	}

	const prefix = "${CLAUDE_PLUGIN_ROOT}/scripts/"
	scriptsDir := filepath.Join("..", "..", "plugin", "claude-code", "scripts")
	validatedCount := 0

	for event, matchers := range raw.Hooks {
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				cmd := hook.Command
				// Search all tokens so commands like "bash ${CLAUDE_PLUGIN_ROOT}/scripts/foo.sh"
				// are also validated, not only commands that start with the prefix.
				var scriptName string
				for _, tok := range strings.Fields(cmd) {
					tok = strings.Trim(tok, `"'`)
					if strings.HasPrefix(tok, prefix) {
						scriptName = strings.TrimPrefix(tok, prefix)
						break
					}
				}
				if scriptName == "" {
					continue
				}
				validatedCount++
				scriptName = filepath.Clean(scriptName)
				if filepath.IsAbs(scriptName) || scriptName == ".." || strings.HasPrefix(scriptName, ".."+string(os.PathSeparator)) {
					t.Errorf("hooks.json [%s]: invalid script path %q", event, scriptName)
					continue
				}
				scriptPath := filepath.Join(scriptsDir, scriptName)
				if _, err := os.Stat(scriptPath); err != nil {
					t.Errorf("hooks.json [%s]: references %q but file does not exist at %s", event, scriptName, scriptPath)
				}
			}
		}
	}
	if validatedCount == 0 {
		t.Fatalf("no script references were validated from hooks.json")
	}
}
