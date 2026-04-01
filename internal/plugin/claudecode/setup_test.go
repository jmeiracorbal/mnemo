package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expectedProtocols = []string{
	"session-start-protocol.md",
	"post-compact-protocol-header.md",
	"post-compact-protocol-footer.md",
	"post-compact-resume-protocol.md",
}

func TestEmbeddedTemplates(t *testing.T) {
	t.Run("mnemo.md", func(t *testing.T) {
		data, err := templatesFS.ReadFile("templates/mnemo.md")
		if err != nil {
			t.Fatalf("template not embedded: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("template is empty")
		}
		content := string(data)
		if !strings.Contains(content, "mem_save") {
			t.Error("template does not contain expected content (mem_save)")
		}
		if !strings.Contains(content, "mem_session_summary") {
			t.Error("template does not contain expected content (mem_session_summary)")
		}
	})
}

func TestRenderTemplate(t *testing.T) {
	content, err := renderTemplate(templatesFS, "templates/mnemo.md", nil)
	if err != nil {
		t.Fatalf("renderTemplate failed: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("rendered content is empty")
	}
	if !strings.Contains(content, "mnemo") {
		t.Error("rendered content does not contain expected text")
	}
}

func TestEmbeddedProtocols(t *testing.T) {
	for _, name := range expectedProtocols {
		t.Run(name, func(t *testing.T) {
			data, err := templatesFS.ReadFile("templates/" + name)
			if err != nil {
				t.Fatalf("protocol template not embedded: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("protocol template is empty")
			}
		})
	}
}

func TestInstallDryRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	i := Installer{}
	if err := i.Install(true); err != nil {
		t.Fatalf("Install dry-run failed: %v", err)
	}
}

// TestShippedHooksReferenceRealScripts validates that every command in
// plugin/claude-code/hooks/hooks.json points to a script that actually
// exists in plugin/claude-code/scripts/. This catches filename mismatches
// before they reach a published release.
func TestShippedHooksReferenceRealScripts(t *testing.T) {
	hooksFile := filepath.Join("..", "..", "..", "plugin", "claude-code", "hooks", "hooks.json")
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
	scriptsDir := filepath.Join("..", "..", "..", "plugin", "claude-code", "scripts")

	for event, matchers := range raw.Hooks {
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				cmd := hook.Command
				if !strings.HasPrefix(cmd, prefix) {
					continue
				}
				scriptName := strings.TrimPrefix(cmd, prefix)
				scriptPath := filepath.Join(scriptsDir, scriptName)
				if _, err := os.Stat(scriptPath); err != nil {
					t.Errorf("hooks.json [%s]: references %q but file does not exist at %s", event, scriptName, scriptPath)
				}
			}
		}
	}
}
