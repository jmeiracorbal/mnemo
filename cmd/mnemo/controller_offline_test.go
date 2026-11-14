package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOfflineRecoveryRequiresNativeManagerConfirmation(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" { t.Skip("Unix service managers only") }
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil { t.Fatal(err) }
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var manager, path, stopped, running string
	if runtime.GOOS == "linux" {
		manager = "systemctl"
		path = filepath.Join(home, ".config", "systemd", "user", controllerSystemdName)
		stopped = "#!/bin/sh\necho inactive\nexit 3\n"
		running = "#!/bin/sh\necho active\nexit 0\n"
	} else {
		manager = "launchctl"
		path = filepath.Join(home, "Library", "LaunchAgents", controllerLaunchLabel+".plist")
		stopped = "#!/bin/sh\necho 'Could not find service' >&2\nexit 3\n"
		running = "#!/bin/sh\necho 'state = running'\nexit 0\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, []byte("installed service"), 0600); err != nil { t.Fatal(err) }
	mock := filepath.Join(binDir, manager)
	if err := os.WriteFile(mock, []byte(stopped), 0755); err != nil { t.Fatal(err) }
	if err := confirmManagedControllerStopped(home); err != nil { t.Fatalf("stopped manager was rejected: %v", err) }
	if err := os.WriteFile(mock, []byte(running), 0755); err != nil { t.Fatal(err) }
	if err := confirmManagedControllerStopped(home); err == nil || !strings.Contains(strings.ToLower(err.Error()), "controller") {
		t.Fatalf("running service was not rejected: %v", err)
	}
}
