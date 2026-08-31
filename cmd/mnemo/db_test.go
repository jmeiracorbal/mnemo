package main

import (
	"path/filepath"
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
