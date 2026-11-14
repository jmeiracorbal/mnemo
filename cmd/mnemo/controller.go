package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/events"
	"github.com/jmeiracorbal/mnemo/internal/procstate"
	"github.com/jmeiracorbal/mnemo/internal/sessionreaper"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

// runController is the installation-owned singleton service. It is never
// started by an MCP runtime.
func runController(s *store.Store) error {
	if len(os.Args) < 3 || os.Args[2] != "serve" {
		return fmt.Errorf("usage: mnemo controller serve")
	}
	cfg, err := events.LoadConfig(s.DataDir())
	if err != nil {
		return err
	}
	controller, err := events.NewController(cfg, s)
	if err != nil {
		return err
	}
	defer controller.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go sessionreaper.Reaper{
		Store:    s,
		Alive: func(pid int) bool {
			return events.UpgradeMaintenance(cfg) || procstate.Alive(pid)
		},
		Interval: orphanedSessionSweepInterval,
		Log:      os.Stderr,
	}.Run(ctx)
	err = controller.Run(ctx)
	if err == nil || !isLegacyCutoverFailure(err) {
		return err
	}
	binary, pathErr := os.Executable()
	if pathErr != nil {
		return fmt.Errorf("legacy delivery failure (cannot find installed executable): %w", err)
	}
	restored, rollbackErr := restorePreviousControllerBinary(binary)
	if rollbackErr != nil {
		return fmt.Errorf("legacy delivery failure (automatic recovery failed: %v): %w", rollbackErr, err)
	}
	if restored {
		_, _ = fmt.Fprintln(os.Stderr, "mnemo controller: incompatible legacy delivery; restored previous Mnemo binary. The service manager will restart it.")
	}
	return err
}

// runControllerPreflight is a read-only verification that is safe to run
// against the previous binary's live JetStream before the installer replaces it.
func runControllerPreflight() error {
	storeConfig, err := store.DefaultConfig()
	if err != nil {
		return err
	}
	cfg, err := events.LoadConfig(storeConfig.DataDir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return events.WaitUpgradeReady(ctx, cfg, 3*time.Second)
}

// runControllerVerify is called by install.sh after a managed-service restart.
// It performs a real read and durable processing check without changing memories.
func runControllerVerify() error {
	storeConfig, err := store.DefaultConfig()
	if err != nil {
		return err
	}
	cfg, err := events.LoadConfig(storeConfig.DataDir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var lastErr error
	for {
		if err := events.VerifyUpgrade(ctx, cfg); err == nil {
			// When SQLite can be restored, legitimate user events remain queued
			// until commit. Re-test durable processing after a short stability
			// observation; do not require the deliberately deferred queue to drain.
			if events.UpgradeMaintenance(cfg) {
				select {
				case <-time.After(2 * time.Second):
					if err := events.VerifyUpgrade(ctx, cfg); err == nil {
						return nil
					} else {
						lastErr = err
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			} else if err := events.WaitUpgradeReady(ctx, cfg, 2*time.Second); err == nil {
				return nil
			} else {
				lastErr = err
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("controller verification failed: %v: %w", lastErr, ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
}

const orphanedSessionSweepInterval = 5 * time.Minute


// isLegacyCutoverFailure identifies the version-boundary failure emitted by
// go-events when an older running publisher sends a headerless JetStream event.
func isLegacyCutoverFailure(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "unsafe go-events cutover") ||
		strings.Contains(message, "invalid event delivery: event ID is required")
}

// restorePreviousControllerBinary is the last line of defense after an
// installer has already returned success. The service manager restarts this
// process with the restored executable, preserving the original JetStream
// messages and SQLite state. Never restore while an installer is still active.
func restorePreviousControllerBinary(binary string) (bool, error) {
	if runtime.GOOS == "windows" {
		return false, nil // The supported shell installer upgrades Unix platforms.
	}
	marker := binary + ".upgrading"
	if content, err := os.ReadFile(marker); err == nil {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(content)))
		if parseErr == nil && pid > 0 {
			process, findErr := os.FindProcess(pid)
			if findErr == nil && process.Signal(syscall.Signal(0)) == nil {
				return false, nil
			}
		}
		if err := os.Remove(marker); err != nil {
			return false, fmt.Errorf("remove abandoned upgrade marker: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect upgrade marker: %w", err)
	}

	// A binary-only downgrade after a schema-changing release could start an
	// older controller against an incompatible database. Never do that.
	if _, err := os.Stat(binary + ".schema-changed"); err == nil {
		return false, fmt.Errorf("cannot restore previous executable after SQLite schema migration without a database recovery transaction")
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect schema compatibility marker: %w", err)
	}
	previous := binary + ".previous"
	info, err := os.Stat(previous)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect previous Mnemo release: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return false, fmt.Errorf("previous Mnemo release is not an executable file")
	}
	if err := os.Rename(previous, binary); err != nil {
		return false, fmt.Errorf("restore previous Mnemo binary: %w", err)
	}
	return true, nil
}


// runControllerStop is invoked by the installer before a SQLite recovery point
// is created. The old controller exits, releasing the database and WAL.
func runControllerStop() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return stopControllerService(home)
}
