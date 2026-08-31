package doctor

import (
	"os"
	"testing"

	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
)

func TestCheckStoreReadOnlyUsesMigratedDatabase(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := dbmigrate.ApplyDataDir(dataDir); err != nil {
		t.Fatalf("prepare migrated database: %v", err)
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
