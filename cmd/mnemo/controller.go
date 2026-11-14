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
		Alive:    procstate.Alive,
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
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return events.VerifyUpgrade(ctx, cfg)
}

const orphanedSessionSweepInterval = 5 * time.Minute
