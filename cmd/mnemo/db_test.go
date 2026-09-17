package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
)

func TestParseDBMigrateArgs(t *testing.T) {
	opts, err := parseDBMigrateArgs([]string{"--data-dir=/tmp/mnemo", "--check", "--json"})
	if err != nil {
		t.Fatalf("parse db migrate args: %v", err)
	}
	if opts.DataDir != "/tmp/mnemo" || !opts.Check || !opts.JSON {
		t.Fatalf("unexpected opts: %+v", opts)
	}
}

func TestParseDBMigrateArgsRejectsUnknown(t *testing.T) {
	if _, err := parseDBMigrateArgs([]string{"--bogus"}); err == nil {
		t.Fatal("expected unknown argument error")
	}
}

func TestGuardControllerNotRunning(t *testing.T) {
	t.Run("no config.toml allows migration", func(t *testing.T) {
		if err := guardControllerNotRunning(t.TempDir()); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("malformed config.toml blocks migration conservatively", func(t *testing.T) {
		dataDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte("not valid toml [[["), 0600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		err := guardControllerNotRunning(dataDir)
		if err == nil {
			t.Fatal("expected error for malformed config.toml, got nil")
		}
		if !strings.Contains(err.Error(), "cannot verify controller state") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("valid config with unreachable controller allows migration", func(t *testing.T) {
		dataDir := t.TempDir()
		// Port 1 is a valid port value that no controller will be listening on.
		cfg := "[events]\nport = 1\n"
		if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(cfg), 0600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := guardControllerNotRunning(dataDir); err != nil {
			t.Fatalf("expected nil for unreachable controller, got %v", err)
		}
	})
}

func TestMigrateDBApplyAndCheck(t *testing.T) {
	dataDir := t.TempDir()
	status, err := migrateDB(dbMigrateOptions{DataDir: dataDir})
	if err != nil {
		t.Fatalf("apply db migrations: %v", err)
	}
	if status.State != dbmigrate.StateApplied {
		t.Fatalf("apply state = %q, want applied", status.State)
	}
	status, err = migrateDB(dbMigrateOptions{DataDir: dataDir, Check: true})
	if err != nil {
		t.Fatalf("check db migrations: %v", err)
	}
	if status.State != dbmigrate.StateUpToDate || status.DBPath != filepath.Join(dataDir, "memory.db") {
		t.Fatalf("check status = %+v", status)
	}
}
