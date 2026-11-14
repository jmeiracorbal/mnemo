package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
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
	if err := os.WriteFile(binary+".previous",[]byte("historical binary"),0755);err!=nil{t.Fatal(err)}
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
	previous,err:=os.ReadFile(binary+".previous")
	if err!=nil || string(previous)!="historical binary"{t.Fatalf("rollback destroyed existing previous slot: %q %v",previous,err)}
	recovered, err := sql.Open("sqlite", filepath.Join(dataDir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = recovered.Close() }()
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
	if err := recordSchemaChange(); err != nil {t.Fatal(err)}
	if err := commitUpgradeTransaction(); err != nil {
		t.Fatal(err)
	}
	if err := recoverControllerStartup(); err != nil {
		t.Fatal(err)
	}
	if _,err:=os.Stat(binary+".schema-changed");err!=nil{t.Fatalf("schema boundary was not recorded: %v",err)}
	previous,err:=os.ReadFile(binary+".previous")
	if err!=nil || string(previous)!="previous binary" {t.Fatalf("committed upgrade lost rollback executable: %q %v",previous,err)}
	got, err := os.ReadFile(binary)
	if err != nil || string(got) != "new binary" {
		t.Fatalf("committed binary unexpectedly reverted: %q %v", got, err)
	}
	current, err := sql.Open("sqlite", filepath.Join(dataDir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = current.Close() }()
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

func TestInterruptedRealMigrationRestoresMigrationHistoryAndMemories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(home, ".mnemo")
	if _, err := dbmigrate.ApplyDataDir(dataDir); err != nil { t.Fatal(err) }
	dbPath := filepath.Join(dataDir, "memory.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil { t.Fatal(err) }
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil { t.Fatal(err) }
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=0"); err != nil { t.Fatal(err) }
	if _, err := db.Exec("INSERT INTO projects(id,name) VALUES ('saved-project','committed before upgrade')"); err != nil { t.Fatal(err) }
	var before int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&before); err != nil { t.Fatal(err) }
	binary := filepath.Join(home, "mnemo")
	backup := binary + ".rollback.stable"
	if err := os.WriteFile(binary, []byte("new"), 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(backup, []byte("old"), 0755); err != nil { t.Fatal(err) }
	if err := beginUpgradeTransaction(binary, backup); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dataDir,"upgrade.maintenance"), []byte("release"),0600);err!=nil{t.Fatal(err)}
	if _, err := recovery.Snapshot(context.Background(), dataDir, "v1.2.0-rollback-1"); err != nil { t.Fatal(err) }
	if err := setUpgradeSnapshot("v1.2.0-rollback-1"); err != nil { t.Fatal(err) }
	if _, err := db.Exec("INSERT INTO schema_migrations(version,name,checksum,dirty) VALUES ('9999','broken','sha256:wrong',0)"); err != nil { t.Fatal(err) }
	if _, err := db.Exec("DELETE FROM projects WHERE id='saved-project'"); err != nil { t.Fatal(err) }
	if err := db.Close(); err != nil { t.Fatal(err) }
	if err := recoverControllerStartup(); err == nil || !strings.Contains(err.Error(),"restarting") { t.Fatalf("recovery failed: %v", err) }
	recovered, err := sql.Open("sqlite", dbPath)
	if err != nil { t.Fatal(err) }
	defer func(){ _ = recovered.Close() }()
	var after int
	if err := recovered.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&after); err != nil { t.Fatal(err) }
	if after != before { t.Fatalf("schema_migrations changed after rollback: before=%d after=%d",before,after) }
	var memories int
	if err := recovered.QueryRow("SELECT COUNT(*) FROM projects WHERE id='saved-project'").Scan(&memories);err!=nil{t.Fatal(err)}
	if memories != 1 {t.Fatal("pre-upgrade committed project was not recovered")}
	if err := recovered.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version='9999'").Scan(&memories); err != nil {t.Fatal(err)}
	if memories != 0 {t.Fatal("failed upgrade migration history survived rollback")}
}

func TestFailedRollbackKeepsWriteFenceAndTransactionJournal(t *testing.T) {
	dataDir, binary, backup, db := transactionFixture(t)
	if err := db.Close(); err != nil { t.Fatal(err) }
	if err := os.Remove(backup); err != nil { t.Fatal(err) }
	if err := recoverControllerStartup(); err == nil || !strings.Contains(err.Error(),"recovery") {
		t.Fatalf("missing old binary did not fail closed: %v",err)
	}
	for _, path := range []string{filepath.Join(dataDir,"upgrade.maintenance"),filepath.Join(dataDir,"upgrade.transaction")} {
		if _,err:=os.Stat(path);err!=nil{t.Fatalf("failed recovery dropped safety marker %s: %v",path,err)}
	}
	current,err:=os.ReadFile(binary)
	if err!=nil || string(current)!="new binary" {t.Fatalf("failed recovery changed active binary: %q %v",current,err)}
}
