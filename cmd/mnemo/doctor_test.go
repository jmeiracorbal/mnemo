package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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

func TestCheckStoreReadOnlyUsesWALSafeImmutableURI(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		closeTestDB(t, db)
		t.Fatalf("enable WAL: %v", err)
	}
	for _, query := range []string{
		"CREATE TABLE sessions (id TEXT)",
		"CREATE TABLE observations (id TEXT)",
		"CREATE TABLE user_prompts (id TEXT)",
		"PRAGMA wal_checkpoint(FULL)",
	} {
		if _, err := db.Exec(query); err != nil {
			closeTestDB(t, db)
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	closeTestDB(t, db)

	uri := sqliteReadOnlyDBURI(dbPath)
	if !strings.Contains(uri, "mode=ro") || !strings.Contains(uri, "immutable=1") {
		t.Fatalf("read-only URI is not WAL-safe: %s", uri)
	}

	if err := os.Chmod(dataDir, 0555); err != nil {
		t.Fatalf("chmod read-only dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dataDir, 0755); err != nil {
			t.Errorf("restore dir permissions: %v", err)
		}
	})

	check := checkStoreReadOnly(dataDir)
	if check.Status != "ok" {
		t.Fatalf("store check status = %q, want ok (message: %s)", check.Status, check.Message)
	}
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

func closeTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("close sqlite: %v", err)
	}
}
