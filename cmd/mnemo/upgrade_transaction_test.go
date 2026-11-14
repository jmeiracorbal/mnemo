package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/db/recovery"
	_ "modernc.org/sqlite"
)

func transactionFixture(t *testing.T) (string, string, string, *sql.DB) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(home, ".mnemo")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE memories (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO memories VALUES ('previously-committed')"); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(home, "mnemo")
	backup := binary + ".rollback.original"
	if err := os.WriteFile(binary, []byte("new binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("previous binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := beginUpgradeTransaction(binary, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "upgrade.maintenance"), []byte("update"), 0600); err != nil {
		t.Fatal(err)
	}
	return dataDir, binary, backup, db
}

func TestInterruptedUpgradeRecoversCommittedWALDataAndExecutable(t *testing.T) {
	dataDir, binary, backup, db := transactionFixture(t)
	if _, err := recovery.Snapshot(context.Background(), dataDir, "v1.1.0-crash-1"); err != nil {
		t.Fatal(err)
	}
	if err := setUpgradeSnapshot("v1.1.0-crash-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO memories VALUES ('uncommitted-release')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recordSchemaChange(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary+".schema-changed", []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	err := recoverControllerStartup()
	if err == nil || !strings.Contains(err.Error(), "restarting") {
		t.Fatalf("interrupted upgrade not recovered: %v", err)
	}
	got, err := os.ReadFile(binary)
	if err != nil || string(got) != "previous binary" {
		t.Fatalf("binary was not restored: %q %v", got, err)
	}
	recovered, err := sql.Open("sqlite", filepath.Join(dataDir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	var count int
	if err := recovered.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("recovery lost committed data or retained speculative changes: %d", count)
	}
	for _, path := range []string{backup, binary + ".schema-changed", binary + ".recovering", binary + ".upgrading", filepath.Join(dataDir, "upgrade.transaction"), filepath.Join(dataDir, "upgrade.maintenance")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("recovery left residue %q: %v", path, err)
		}
	}
}

func TestCommittedUpgradeNeverReplaysOldSnapshot(t *testing.T) {
	dataDir, binary, backup, db := transactionFixture(t)
	if _, err := recovery.Snapshot(context.Background(), dataDir, "v1.1.0-committed-1"); err != nil {
		t.Fatal(err)
	}
	if err := setUpgradeSnapshot("v1.1.0-committed-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO memories VALUES ('acknowledged-after-commit')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := commitUpgradeTransaction(); err != nil {
		t.Fatal(err)
	}
	if err := recoverControllerStartup(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(binary)
	if err != nil || string(got) != "new binary" {
		t.Fatalf("committed binary unexpectedly reverted: %q %v", got, err)
	}
	current, err := sql.Open("sqlite", filepath.Join(dataDir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	var count int
	if err := current.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("committed memory was lost: %d", count)
	}
	for _, path := range []string{backup, filepath.Join(dataDir, "upgrade.transaction"), filepath.Join(dataDir, "upgrade.maintenance")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("committed cleanup left %q: %v", path, err)
		}
	}
}

func TestInterruptedUpgradeBeforeSnapshotRestoresOnlyExecutable(t *testing.T) {
	dataDir, binary, _, db := transactionFixture(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recoverControllerStartup(); err == nil || !strings.Contains(err.Error(), "restarting") {
		t.Fatalf("interrupted update not recovered: %v", err)
	}
	got, err := os.ReadFile(binary)
	if err != nil || string(got) != "previous binary" {
		t.Fatalf("binary changed despite recovery: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "upgrade.maintenance")); !os.IsNotExist(err) {
		t.Fatalf("stale write fence remains: %v", err)
	}
}
