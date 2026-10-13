package main

import (
	"os"
	"path/filepath"
	"testing"
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
