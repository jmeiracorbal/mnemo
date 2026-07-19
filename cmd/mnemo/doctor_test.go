package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

func TestParseDoctorArgs(t *testing.T) {
	opts := parseDoctorArgs([]string{"--json", "--agent=codex", "--path=/repo", "--home=/home/test", "--data-dir=/data"})
	if !opts.JSON || opts.Agent != "codex" || opts.Path != "/repo" || opts.Home != "/home/test" || opts.DataDir != "/data" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestBuildDoctorReportChecksCodexGlobalInstall(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	dataDir := t.TempDir()

	if err := agentinit.EnsureMarkerWithID(project, "project-123"); err != nil {
		t.Fatalf("marker: %v", err)
	}
	if _, err := agentinit.InstallGlobalInstructions(home, "codex"); err != nil {
		t.Fatalf("install instructions: %v", err)
	}
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.mnemo]\ncommand = \"mnemo\"\nargs = [\"mcp\", \"--tools=agent\"]\n")
	writeFile(t, filepath.Join(home, ".codex", "hooks.json"), `{"hooks":{"SessionStart":[],"Stop":[]}}`)
	writeExecutable(t, filepath.Join(home, ".codex", "hooks", "session-start.sh"))
	writeExecutable(t, filepath.Join(home, ".codex", "hooks", "stop.sh"))
	writeFile(t, filepath.Join(home, ".codex", "hooks", "mnemo-protocol.md"), "protocol")

	report := buildDoctorReport(doctorOptions{Agent: "codex", Path: project, Home: home, DataDir: dataDir})

	assertCheckStatus(t, report, "project_marker", "ok")
	assertCheckStatus(t, report, "global_instructions.codex", "ok")
	assertCheckStatus(t, report, "mcp_config.codex", "ok")
	assertCheckStatus(t, report, "runtime_files.codex", "ok")
}

func TestBuildDoctorReportWarnsWhenProjectMarkerMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	report := buildDoctorReport(doctorOptions{Agent: "codex", Path: project, Home: home, DataDir: t.TempDir()})
	assertCheckStatus(t, report, "project_marker", "warning")
}

func assertCheckStatus(t *testing.T, report doctorReport, id, want string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			if check.Status != want {
				t.Fatalf("check %s status = %q, want %q (message: %s)", id, check.Status, want, check.Message)
			}
			return
		}
	}
	t.Fatalf("check %s not found in %+v", id, report.Checks)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
}
