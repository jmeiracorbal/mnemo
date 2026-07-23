package main

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSetupPrintConfigArgs(t *testing.T) {
	opts, err := parseSetupPrintConfigArgs(
		[]string{"codex", "--home=/home/test", "--mnemo-bin=/bin/mnemo"},
		func() (string, error) {
			t.Fatal("userHomeDir should not be called when --home is provided")
			return "", nil
		},
		func(string) (string, error) {
			t.Fatal("lookPath should not be called when --mnemo-bin is provided")
			return "", nil
		},
	)
	if err != nil {
		t.Fatalf("parse setup print-config args: %v", err)
	}
	if opts.Agent != "codex" || opts.Home != "/home/test" || opts.MnemoBin != "/bin/mnemo" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseSetupPrintConfigArgsDefaultsBinaryToPath(t *testing.T) {
	opts, err := parseSetupPrintConfigArgs(
		[]string{"cursor"},
		func() (string, error) { return "/home/test", nil },
		func(name string) (string, error) {
			if name != "mnemo" {
				t.Fatalf("lookPath called with %q", name)
			}
			return "/usr/local/bin/mnemo", nil
		},
	)
	if err != nil {
		t.Fatalf("parse setup print-config args: %v", err)
	}
	if opts.Agent != "cursor" || opts.Home != "/home/test" || opts.MnemoBin != "/usr/local/bin/mnemo" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseSetupPrintConfigArgsDefaultsBinaryToMnemo(t *testing.T) {
	opts, err := parseSetupPrintConfigArgs(
		[]string{"cursor"},
		func() (string, error) { return "/home/test", nil },
		func(string) (string, error) { return "", errors.New("not found") },
	)
	if err != nil {
		t.Fatalf("parse setup print-config args: %v", err)
	}
	if opts.MnemoBin != "mnemo" {
		t.Fatalf("mnemo bin = %q, want mnemo", opts.MnemoBin)
	}
}

func TestBuildSetupConfigSnippetsForCodex(t *testing.T) {
	snippets, err := buildSetupConfigSnippets(setupPrintConfigOptions{
		Agent:    "codex",
		Home:     "/home/test",
		MnemoBin: "/bin/mnemo",
	})
	if err != nil {
		t.Fatalf("build snippets: %v", err)
	}
	if len(snippets) != 2 {
		t.Fatalf("snippets = %d, want 2", len(snippets))
	}
	if snippets[0].Path != "/home/test/.codex/config.toml" || !strings.Contains(snippets[0].Content, "[mcp_servers.mnemo]") || !strings.Contains(snippets[0].Content, `command = "/bin/mnemo"`) {
		t.Fatalf("unexpected mcp snippet: %+v", snippets[0])
	}
	if snippets[1].Path != "/home/test/.codex/hooks.json" || !strings.Contains(snippets[1].Content, "/home/test/.codex/hooks/session-start.sh") {
		t.Fatalf("unexpected hooks snippet: %+v", snippets[1])
	}
}

func TestBuildSetupConfigSnippetsForAll(t *testing.T) {
	snippets, err := buildSetupConfigSnippets(setupPrintConfigOptions{
		Agent:    "all",
		Home:     "/home/test",
		MnemoBin: "mnemo",
	})
	if err != nil {
		t.Fatalf("build snippets: %v", err)
	}
	if len(snippets) != 8 {
		t.Fatalf("snippets = %d, want 8", len(snippets))
	}
}
