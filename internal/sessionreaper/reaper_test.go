package sessionreaper

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	mu     sync.Mutex
	sweeps int
	result int
	err    error
}

func (f *fakeStore) CloseOrphanedMCPSessions(alive func(pid int) bool) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sweeps++
	return f.result, f.err
}

func (f *fakeStore) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sweeps
}

func TestReaperSweepsAtStartAndPeriodicallyUntilCancelled(t *testing.T) {
	store := &fakeStore{result: 2}
	var log bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Reaper{Store: store, Alive: func(int) bool { return false }, Interval: 10 * time.Millisecond, Log: &log}.Run(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for store.count() < 3 {
		select {
		case <-deadline:
			t.Fatalf("sweeps = %d, want at least 3", store.count())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reaper did not stop after cancellation")
	}
	if !strings.Contains(log.String(), "closed sessions of 2 exited MCP servers") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestReaperReportsErrorsAndStaysSilentWithoutWork(t *testing.T) {
	var log bytes.Buffer
	Reaper{Store: &fakeStore{}, Log: &log}.sweep()
	if log.Len() != 0 {
		t.Fatalf("sweep without work logged %q", log.String())
	}
	Reaper{Store: &fakeStore{err: errors.New("db locked")}, Log: &log}.sweep()
	if !strings.Contains(log.String(), "db locked") {
		t.Fatalf("log = %q, want error", log.String())
	}
}
