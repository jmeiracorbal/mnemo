package doctor

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbmigrate "github.com/jmeiracorbal/mnemo/v2/internal/db/migrate"
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

func TestCheckControllerHealthReportsUnavailableController(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve controller port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release controller port: %v", err)
	}

	dataDir := t.TempDir()
	config := fmt.Sprintf("[events]\nport = %d\n", port)
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatalf("write event config: %v", err)
	}

	check := checkControllerHealth(dataDir, "codex")
	if check.Status != "error" {
		t.Fatalf("controller check status = %q, want error (message: %s)", check.Status, check.Message)
	}
	if !strings.Contains(check.Message, "controller is unavailable") {
		t.Fatalf("controller check message = %q, want unavailable diagnosis", check.Message)
	}
	if check.Fix != "mnemo doctor --fix --agent=codex" {
		t.Fatalf("controller check fix = %q, want doctor repair command", check.Fix)
	}
}
