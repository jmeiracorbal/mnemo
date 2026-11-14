package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

func TestEnsureControllerServiceRestartsOnlyWhenNeeded(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "mnemo")
	if err := os.WriteFile(bin, []byte("binary v1"), 0755); err != nil {
		t.Fatal(err)
	}
	unitPath := filepath.Join(home, "Library", "LaunchAgents", "controller.plist")
	isRunning := true
	restarts := 0
	running := func() bool { return isRunning }
	restart := func() error { restarts++; isRunning = true; return nil }
	ensure := func(unit string) bool {
		t.Helper()
		_, restarted, err := ensureControllerService(home, bin, unitPath, unit, running, restart)
		if err != nil {
			t.Fatalf("ensure controller service: %v", err)
		}
		return restarted
	}

	if !ensure("unit v1") {
		t.Fatal("first install did not start the service")
	}
	for range 2 {
		if ensure("unit v1") {
			t.Fatal("unchanged service restarted")
		}
	}
	if !ensure("unit v2") {
		t.Fatal("changed service definition did not restart")
	}
	if err := os.WriteFile(bin, []byte("binary v2"), 0755); err != nil {
		t.Fatal(err)
	}
	if !ensure("unit v2") {
		t.Fatal("changed binary did not restart")
	}
	isRunning = false
	if !ensure("unit v2") {
		t.Fatal("stopped service was not started")
	}
	if ensure("unit v2") {
		t.Fatal("running, unchanged service restarted")
	}
	if restarts != 4 {
		t.Fatalf("restarts = %d, want 4", restarts)
	}
}

func TestStoreCommandPropagatesControllerError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fatal := errors.New("durable transport failed")
	cmd := storeCommand("serve", "test controller", func(*store.Store) error {
		return fatal
	})
	cmd.SetArgs(nil)
	if err := cmd.Execute(); !errors.Is(err, fatal) {
		t.Fatalf("controller exit error = %v; want %v", err, fatal)
	}
}

func TestPreviousControllerBinaryIsRestoredAfterLateLegacyEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mnemo")
	if err := os.WriteFile(path, []byte("new release"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".previous", []byte("previous release"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isLegacyCutoverFailure(errors.New("durable transport failure: invalid event delivery: event ID is required")) {
		t.Fatal("late headerless event must trigger automatic recovery")
	}
	if !isLegacyCutoverFailure(errors.New("unsafe go-events cutover: old message")) {
		t.Fatal("legacy event found on service restart must trigger recovery")
	}
	if isLegacyCutoverFailure(errors.New("ordinary transient network failure")) {
		t.Fatal("unrelated transport failures must not downgrade Mnemo")
	}
	restored, err := restorePreviousControllerBinary(path)
	if err != nil || !restored {
		t.Fatalf("restore previous controller: restored=%v err=%v", restored, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "previous release" {
		t.Fatalf("previous binary was not restored: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(path+".previous"); !os.IsNotExist(err) {
		t.Fatalf("recovery slot should be consumed: %v", err)
	}
}

func TestRuntimeRollbackWaitsForActiveInstaller(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mnemo")
	for _, entry := range []struct{ path, contents string }{
		{path, "new release"},
		{path + ".previous", "previous release"},
		{path + ".upgrading", strconv.Itoa(os.Getpid())},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.contents), 0755); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := restorePreviousControllerBinary(path)
	if err != nil || restored {
		t.Fatalf("active installer must own rollback: restored=%v err=%v", restored, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "new release") {
		t.Fatalf("binary replaced while installer was active: %v", err)
	}
}

func TestRuntimeRollbackRefusesIncompatibleSQLiteSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mnemo")
	for _, entry := range []struct{ name, value string }{
		{path, "new binary"},
		{path + ".previous", "previous binary"},
		{path + ".schema-changed", "schema migration committed"},
	} {
		if err := os.WriteFile(entry.name, []byte(entry.value), 0755); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := restorePreviousControllerBinary(path)
	if restored || err == nil || !strings.Contains(err.Error(), "SQLite schema migration") {
		t.Fatalf("unsafe binary-only downgrade was not blocked: restored=%v err=%v", restored, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new binary" {
		t.Fatalf("active binary was modified despite incompatible schema: %q %v", data, err)
	}
}
