package main

import (
	"errors"
	"testing"
)

func TestParseInstallInstructionsArgsHomeOverrideDoesNotRequireUserHome(t *testing.T) {
	agent, home, err := parseInstallInstructionsArgs(
		[]string{"--agent=codex", "--home=/tmp/mnemo-home"},
		func() (string, error) { return "", errors.New("HOME not set") },
	)
	if err != nil {
		t.Fatalf("expected --home to avoid user home lookup error, got %v", err)
	}
	if agent != "codex" {
		t.Fatalf("agent = %q, want codex", agent)
	}
	if home != "/tmp/mnemo-home" {
		t.Fatalf("home = %q, want /tmp/mnemo-home", home)
	}
}

func TestParseInstallInstructionsArgsFallsBackToUserHome(t *testing.T) {
	agent, home, err := parseInstallInstructionsArgs(
		[]string{"--agent=opencode"},
		func() (string, error) { return "/home/tester", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent != "opencode" {
		t.Fatalf("agent = %q, want opencode", agent)
	}
	if home != "/home/tester" {
		t.Fatalf("home = %q, want /home/tester", home)
	}
}
