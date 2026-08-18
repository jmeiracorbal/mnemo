package doctor

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func closeTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("close sqlite: %v", err)
	}
}
