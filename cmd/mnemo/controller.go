package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmeiracorbal/mnemo-events"
	"github.com/jmeiracorbal/mnemo/internal/procstate"
	"github.com/jmeiracorbal/mnemo/internal/sessionreaper"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

// runController is the installation-owned singleton service. It is never
// started by an MCP runtime.
func runController(s *store.Store) {
	if len(os.Args) < 3 || os.Args[2] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: mnemo controller serve")
		return
	}
	cfg, err := events.LoadConfig(s.DataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo controller: %v\n", err)
		return
	}
	controller, err := events.NewController(cfg, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo controller: %v\n", err)
		return
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
	if err := controller.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo controller: %v\n", err)
	}
}

const orphanedSessionSweepInterval = 5 * time.Minute
