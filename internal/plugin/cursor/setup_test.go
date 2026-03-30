package cursor

import (
	"strings"
	"testing"
)

var expectedScripts = []string{
	"before-submit-prompt.sh",
	"stop.sh",
}

func TestEmbeddedScripts(t *testing.T) {
	for _, name := range expectedScripts {
		t.Run(name, func(t *testing.T) {
			data, err := scriptsFS.ReadFile("scripts/" + name)
			if err != nil {
				t.Fatalf("script not embedded: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("script is empty")
			}
			if !strings.HasPrefix(string(data), "#!/bin/bash") {
				t.Fatalf("script missing shebang, starts with: %q", string(data[:min(40, len(data))]))
			}
		})
	}
}

func TestEmbeddedTemplates(t *testing.T) {
	t.Run("mnemo.mdc", func(t *testing.T) {
		data, err := templatesFS.ReadFile("templates/mnemo.mdc")
		if err != nil {
			t.Fatalf("template not embedded: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("template is empty")
		}
		content := string(data)
		if !strings.Contains(content, "alwaysApply: true") {
			t.Error("template missing Cursor MDC frontmatter (alwaysApply)")
		}
		if !strings.Contains(content, "mem_save") {
			t.Error("template does not contain expected content (mem_save)")
		}
		if !strings.Contains(content, "mem_session_summary") {
			t.Error("template does not contain expected content (mem_session_summary)")
		}
	})
}

func TestRenderTemplate(t *testing.T) {
	content, err := renderTemplate(templatesFS, "templates/mnemo.mdc", nil)
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

func TestHookScriptsList(t *testing.T) {
	if len(hookScripts) != len(expectedScripts) {
		t.Errorf("hookScripts has %d entries, want %d", len(hookScripts), len(expectedScripts))
	}
	nameSet := make(map[string]bool, len(hookScripts))
	for _, name := range hookScripts {
		nameSet[name] = true
	}
	for _, name := range expectedScripts {
		if !nameSet[name] {
			t.Errorf("hookScripts missing expected entry: %s", name)
		}
	}
}

func TestInstallDryRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	i := Installer{}
	if err := i.Install(true); err != nil {
		t.Fatalf("Install dry-run failed: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
