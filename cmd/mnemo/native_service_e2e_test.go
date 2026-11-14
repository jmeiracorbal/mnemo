package main

import (
    "bytes"
    "context"
    "crypto/sha256"
    "database/sql"
    "fmt"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "testing"
    "time"

    _ "modernc.org/sqlite"
)

// This test exercises a real launchd/systemd user controller, not a substituted
// service manager. Only the dedicated native-service CI job enables it.
func TestNativeServiceUpgradeRollback(t *testing.T) {
    if os.Getenv("MNEMO_NATIVE_SERVICE_E2E") != "1" {
        t.Skip("native service integration runs in the dedicated CI job")
    }
    if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
        t.Fatal("native service test requires launchd or systemd")
    }
    oldBin := os.Getenv("MNEMO_NATIVE_OLD_BIN")
    newBin := os.Getenv("MNEMO_NATIVE_NEW_BIN")
    if oldBin == "" || newBin == "" {
        t.Fatal("native executable paths are required")
    }
    home, err := os.UserHomeDir()
    if err != nil { t.Fatal(err) }
    dataDir := filepath.Join(home, ".mnemo")
    bin := filepath.Join(t.TempDir(), "mnemo")
    if err := copyExecutable(oldBin, bin); err != nil { t.Fatal(err) }

    // A real manager controls the service lifetime. CI runners are ephemeral;
    // no test-only manager or fake health endpoint is installed.
    t.Cleanup(func() {
        _ = nativeTestRun(bin, "controller", "stop")
        switch runtime.GOOS {
        case "darwin":
            path := filepath.Join(home, "Library", "LaunchAgents", controllerLaunchLabel+".plist")
            _ = exec.Command("launchctl", "bootout", "gui/"+strconv.Itoa(os.Getuid()), path).Run()
        case "linux":
            _ = exec.Command("systemctl", "--user", "disable", "--now", controllerSystemdName).Run()
        }
    })
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil { t.Fatal(err) }
    port := listener.Addr().(*net.TCPAddr).Port
    if err := listener.Close(); err != nil { t.Fatal(err) }
    if err := os.MkdirAll(dataDir, 0700); err != nil { t.Fatal(err) }
    config := fmt.Sprintf("[events]\nport = %d\n", port)
    if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(config), 0600); err != nil { t.Fatal(err) }

    serviceRefresh := func(executable string) {
        t.Helper()
        if err := nativeTestRun(executable, "setup", "refresh", "--agent=codex", "--mnemo-bin="+bin); err != nil {
            t.Fatalf("real service refresh: %v", err)
        }
        if err := nativeTestRun(executable, "controller", "verify"); err != nil {
            t.Fatalf("real service did not become ready: %v; log: %s", err, nativeTestLog(dataDir))
        }
    }
    serviceRefresh(oldBin)

    dbPath := filepath.Join(dataDir, "memory.db")
    db, err := sql.Open("sqlite", dbPath)
    if err != nil { t.Fatal(err) }
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil { t.Fatal(err) }
    if _, err := db.Exec("INSERT INTO projects (id, name) VALUES ('native-service-test', 'committed-before-upgrade')"); err != nil { t.Fatal(err) }
    if err := db.Close(); err != nil { t.Fatal(err) }
    before := nativeTestCount(t, dbPath)
    if before != 1 { t.Fatalf("committed seed missing: %d", before) }

    begin := func(release string) string {
        t.Helper()
        backup := bin + ".rollback." + release
        if err := copyExecutable(bin, backup); err != nil { t.Fatal(err) }
        if err := nativeTestRun(newBin, "controller", "transaction", "begin", "--binary="+bin, "--backup="+backup); err != nil { t.Fatal(err) }
        // The fixture is the actual installer owner for the subsequent forced
        // recovery, so the Go child process sees this test's OS PID as parent.
        marker := bin + ".upgrading"
        if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil { t.Fatal(err) }
        if err := os.WriteFile(filepath.Join(dataDir, "upgrade.maintenance"), []byte(release), 0600); err != nil { t.Fatal(err) }
        if err := nativeTestRun(newBin, "controller", "stop"); err != nil { t.Fatalf("real service stop: %v", err) }
        if err := confirmManagedControllerStopped(home); err != nil { t.Fatalf("native manager still active: %v", err) }
        if err := nativeTestRun(newBin, "db", "recovery", "snapshot", "--release="+release); err != nil { t.Fatal(err) }
        if err := nativeTestRun(newBin, "controller", "transaction", "snapshot", "--release="+release); err != nil { t.Fatal(err) }
        return backup
    }

    // Failure: the old SQLite data must be recovered with its binary before
    // this manager can restart or acknowledge later events.
    rollbackRelease := "v1.0.0-native-rollback"
    begin(rollbackRelease)
    database, err := sql.Open("sqlite", dbPath)
    if err != nil { t.Fatal(err) }
    if _, err := database.Exec("DELETE FROM projects WHERE id = 'native-service-test'"); err != nil { t.Fatal(err) }
    if err := database.Close(); err != nil { t.Fatal(err) }
    if err := copyExecutable(newBin, bin); err != nil { t.Fatal(err) }
    serviceRefresh(newBin)
    if err := nativeTestRun(newBin, "controller", "stop"); err != nil { t.Fatal(err) }
    if err := nativeTestRun(newBin, "controller", "transaction", "recover", "--force"); err != nil { t.Fatalf("real rollback: %v", err) }
    if got := nativeTestCount(t, dbPath); got != 1 { t.Fatalf("rollback lost committed row: got %d", got) }
    oldHash := nativeTestHash(t, oldBin)
    if got := nativeTestHash(t, bin); got != oldHash { t.Fatal("recovery failed to restore previous executable") }
    serviceRefresh(oldBin)

    // Success: the same manager switches to the new binary, verifies it and
    // commits the journal. Existing committed SQLite rows remain intact.
    commitRelease := "v1.0.0-native-commit"
    begin(commitRelease)
    if err := copyExecutable(newBin, bin); err != nil { t.Fatal(err) }
    serviceRefresh(newBin)
    if err := nativeTestRun(newBin, "controller", "transaction", "commit"); err != nil { t.Fatal(err) }
    if err := nativeTestRun(newBin, "controller", "transaction", "finalize"); err != nil { t.Fatal(err) }
    if got := nativeTestHash(t, bin); got != nativeTestHash(t, newBin) { t.Fatal("new executable not activated") }
    if got := nativeTestHash(t, bin+".previous"); got != oldHash { t.Fatal("previous executable slot not preserved") }
    if got := nativeTestCount(t, dbPath); got != 1 { t.Fatalf("committed SQLite row changed: %d", got) }
    for _, marker := range []string{filepath.Join(dataDir, "upgrade.transaction"), filepath.Join(dataDir, "upgrade.maintenance"), bin+".upgrading"} {
        if _, err := os.Stat(marker); !os.IsNotExist(err) { t.Fatalf("transaction residue %s: %v", marker, err) }
    }
    if err := nativeTestRun(newBin, "controller", "verify"); err != nil { t.Fatalf("committed service unhealthy: %v", err) }
}

func nativeTestRun(binary string, args ...string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, binary, args...)
    cmd.Env = append(os.Environ(), "MNEMO_NO_UPDATE_CHECK=1")
    var output bytes.Buffer
    cmd.Stdout = &output
    cmd.Stderr = &output
    if err := cmd.Run(); err != nil { return fmt.Errorf("%s %s: %w: %s", binary, strings.Join(args, " "), err, output.String()) }
    return nil
}

func copyExecutable(src, dest string) error {
    content, err := os.ReadFile(src)
    if err != nil { return err }
    pending := dest+".activate"
    if err := os.WriteFile(pending, content, 0755); err != nil { return err }
    return os.Rename(pending, dest)
}

func nativeTestCount(t *testing.T, path string) int {
    t.Helper()
    db, err := sql.Open("sqlite", path)
    if err != nil { t.Fatal(err) }
    defer func() { _ = db.Close() }()
    var count int
    if err := db.QueryRow("SELECT COUNT(*) FROM projects WHERE id = 'native-service-test'").Scan(&count); err != nil { t.Fatal(err) }
    return count
}

func nativeTestHash(t *testing.T, path string) [32]byte {
    t.Helper()
    contents, err := os.ReadFile(path)
    if err != nil { t.Fatal(err) }
    return sha256.Sum256(contents)
}

func nativeTestLog(dataDir string) string {
    c, err := os.ReadFile(filepath.Join(dataDir, "controller.err.log"))
    if err != nil { return err.Error() }
    if len(c) > 5000 { c = c[len(c)-5000:] }
    return string(c)
}
