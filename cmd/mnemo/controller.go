package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
	return controller.Run(ctx)
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


// runControllerStop is invoked by the installer before a SQLite recovery point
// is created. The old controller exits, releasing the database and WAL.
func runControllerStop() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return stopControllerService(home)
}
