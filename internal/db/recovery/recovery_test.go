package recovery

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openRecoveryTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	return db
}

func recoveryCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM saved").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestSnapshotIncludesCommittedWALAndRestoreRemovesNewerWrites(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")
	db := openRecoveryTestDB(t, dbPath)
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE saved (value TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO saved(value) VALUES ('committed-before-update')"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	point, err := Snapshot(ctx, dataDir, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if point != filepath.Join(dataDir, "recovery", "v1.2.0.db") {
		t.Fatalf("unexpected recovery point %s", point)
	}
	if err := Verify(ctx, dataDir, "v1.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO saved(value) VALUES ('write-after-snapshot')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, dataDir, "v1.2.0"); err != nil {
		t.Fatal(err)
	}
	recovered := openRecoveryTestDB(t, dbPath)
	defer func() { _ = recovered.Close() }()
	if count := recoveryCount(t, recovered); count != 1 {
		t.Fatalf("recovered %d rows, expected one committed pre-upgrade row", count)
	}
}

func TestInvalidRestoreDoesNotReplaceCurrentDatabase(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")
	db := openRecoveryTestDB(t, dbPath)
	if _, err := db.Exec("CREATE TABLE saved (value TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO saved(value) VALUES ('original')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dataDir, "recovery")
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "v2.0.0.db"), []byte("invalid sqlite database"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(context.Background(), dataDir, "v2.0.0"); err == nil {
		t.Fatal("corrupt recovery point was accepted")
	}
	original := openRecoveryTestDB(t, dbPath)
	defer func() { _ = original.Close() }()
	if count := recoveryCount(t, original); count != 1 {
		t.Fatalf("original database changed after rejected restore: %d", count)
	}
}

func TestRecoveryIDsCannotEscapeDataDir(t *testing.T) {
	for _, id := range []string{"../parent", "v1.2.0/../../user", "", "latest", "v1.0.0\\malicious"} {
		if _, err := pointPath(t.TempDir(), id); err == nil {
			t.Errorf("accepted invalid release identifier %q", id)
		}
	}
}

func TestSnapshotFailsClosedWhenRecoveryPointIsPending(t *testing.T) {
	dataDir := t.TempDir()
	db := openRecoveryTestDB(t, filepath.Join(dataDir, "memory.db"))
	if _, err := db.Exec("CREATE TABLE saved (value TEXT)"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	p := filepath.Join(dataDir, "recovery")
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "v1.2.0.db.pending"), []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Snapshot(context.Background(), dataDir, "v1.2.0")
	if err == nil || !strings.Contains(err.Error(), "reserve recovery point") {
		t.Fatalf("pending snapshot must block a new snapshot: %v", err)
	}
}

func TestRecoveryPointCannotBeOverwrittenForSameRelease(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")
	db := openRecoveryTestDB(t, dbPath)
	if _, err := db.Exec("CREATE TABLE saved (value TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO saved(value) VALUES ('before')"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := Snapshot(ctx, dataDir, "v1.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO saved(value) VALUES ('after')"); err != nil {
		t.Fatal(err)
	}
	if _, err := Snapshot(ctx, dataDir, "v1.2.0"); err == nil {
		t.Fatal("existing release recovery point must never be overwritten")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, dataDir, "v1.2.0"); err != nil {
		t.Fatal(err)
	}
	recovered := openRecoveryTestDB(t, dbPath)
	defer func() { _ = recovered.Close() }()
	if got := recoveryCount(t, recovered); got != 1 {
		t.Fatalf("immutable restore point was overwritten: got %d rows", got)
	}
}
