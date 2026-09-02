package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandContainsEveryMenuCommand(t *testing.T) {
	root := newRootCommand()

	want := []string{
		"mcp", "save", "search", "context", "session", "stats", "export", "import", "capture",
		"projects", "memories", "doctor", "db", "update", "setup", "init", "install-instructions",
		"migrate", "sync", "json", "json-merge", "extract-transcript", "version",
	}
	for _, name := range want {
		if findChild(root, name) == nil {
			t.Fatalf("root command %q is missing", name)
		}
	}
}

func TestRootCommandNestedMenu(t *testing.T) {
	root := newRootCommand()
	checks := map[string][]string{
		"db":       {"migrate"},
		"setup":    {"status", "print-config", "refresh", "uninstall", "codex", "all"},
		"projects": {"list", "merge", "rename"},
		"memories": {"review", "mark-reviewed", "mark-stale", "supersede", "consolidate-topic"},
		"session":  {"start", "end", "exists", "obs-count", "project-obs-count"},
		"sync":     {"run", "push", "pull", "status"},
	}
	for parentName, children := range checks {
		parent, _, err := root.Find([]string{parentName})
		if err != nil {
			t.Fatalf("find %s: %v", parentName, err)
		}
		for _, childName := range children {
			if parent.CommandPath() == "" || findChild(parent, childName) == nil {
				t.Errorf("nested command %s %s is missing", parentName, childName)
			}
		}
	}
}

func TestLeafHelpIsGeneratedWithoutOpeningStore(t *testing.T) {
	root := newRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"save", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	if !strings.Contains(out.String(), "Save a memory") || !strings.Contains(out.String(), "mnemo save") {
		t.Fatalf("unexpected generated help:\n%s", out.String())
	}
}

func TestAllMenuNodesHaveHelp(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Name() == "completion" {
			return
		}
		child := newRootCommand()
		var out bytes.Buffer
		child.SetOut(&out)
		child.SetErr(&out)
		args := strings.Fields(cmd.CommandPath())[1:]
		args = append(args, "--help")
		child.SetArgs(args)
		if err := child.Execute(); err != nil {
			t.Errorf("help failed for %s: %v", cmd.CommandPath(), err)
		}
		for _, nested := range cmd.Commands() {
			walk(nested)
		}
	}
	walk(newRootCommand())
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}
