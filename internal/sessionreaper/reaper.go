// Package sessionreaper closes mnemo sessions whose MCP server exited without
// closing them. It runs inside the controller, which owns session lifecycle
// and SQLite writes.
package sessionreaper

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Store closes the open sessions of MCP servers that alive reports as gone,
// returning how many MCP instances it closed.
type Store interface {
	CloseOrphanedMCPSessions(alive func(pid int) bool) (int, error)
}

// Reaper sweeps orphaned MCP sessions at start and then every Interval.
type Reaper struct {
	Store    Store
	Alive    func(pid int) bool
	Interval time.Duration
	Log      io.Writer
}

// Run sweeps until ctx is cancelled.
func (r Reaper) Run(ctx context.Context) {
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	for {
		r.sweep()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r Reaper) sweep() {
	closed, err := r.Store.CloseOrphanedMCPSessions(r.Alive)
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(r.Log, "mnemo controller: close orphaned MCP sessions: %v\n", err)
	case closed > 0:
		_, _ = fmt.Fprintf(r.Log, "mnemo controller: closed sessions of %d exited MCP servers\n", closed)
	}
}
