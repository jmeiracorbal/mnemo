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

func TestShippedProtocolsForbidFallbackMemory(t *testing.T) {
	files := []string{
		filepath.Join("..", "..", "templates", "rules", "generic.md"),
		filepath.Join("..", "..", "templates", "rules", "global.md"),
		filepath.Join("..", "..", "templates", "rules", "cursor.mdc"),
		filepath.Join("..", "..", "templates", "rules", "cursor-global.mdc"),
		filepath.Join("..", "..", "templates", "rules", "windsurf.md"),
		filepath.Join("..", "..", "plugin", "claude-code", "scripts", "mnemo.md"),
		filepath.Join("..", "..", "plugin", "claude-code", "scripts", "session-start-protocol.md"),
		filepath.Join("..", "..", "plugin", "claude-code", "scripts", "post-compact-protocol-header.md"),
		filepath.Join("..", "..", "scripts", "codex", "hooks", "mnemo-protocol.md"),
		filepath.Join("..", "..", "scripts", "cursor", "rules", "mnemo.mdc"),
		filepath.Join("..", "..", "scripts", "windsurf", "templates", "global_rules.md"),
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "mnemo is the ONLY persistent memory system") {
			t.Errorf("%s does not declare mnemo as the only persistent memory", path)
		}
		if !strings.Contains(content, "plaintext files as a memory fallback") {
			t.Errorf("%s does not forbid plaintext memory fallback", path)
		}
	}
}

func TestShippedMnemoMemorySkill(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "mnemo-memory", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped skill: %v", err)
	}
	content := string(data)
	required := []string{
		"name: mnemo-memory",
		"description:",
		"<root>/.mnemo",
		"non-empty `id`",
		"Never create `MEMORY.md`",
		"`mem_session_summary`",
	}
	for _, value := range required {
		if !strings.Contains(content, value) {
			t.Errorf("shipped skill missing %q", value)
		}
	}
}
