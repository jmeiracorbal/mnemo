package events

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

func TestControllerAppliesEventExactlyOnceAfterExecutionBinding(t *testing.T) {
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })

	project := "project-events"
	executionKey := "execution-key"
	sessionID, err := memory.ResolveMCPInstanceSession(project, t.TempDir(), "mcp-instance", 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := memory.BindExecutionSession(project, executionKey, sessionID); err != nil {
		t.Fatalf("bind execution session: %v", err)
	}

	cfg := Config{Project: project, Port: freePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, memory)
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- controller.Run(ctx) }()

	event, err := NewEvent(EventWorkspaceFileChanged, project, executionKey, map[string]string{"path": "internal/events/events.go", "action": "modified"})
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	if err := Publish(context.Background(), cfg, event); err != nil {
		t.Fatalf("publish first event: %v", err)
	}
	if err := Publish(context.Background(), cfg, event); err != nil {
		t.Fatalf("publish duplicate event: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		observations, err := memory.SessionObservations(sessionID, 10)
		if err != nil {
			t.Fatalf("list session observations: %v", err)
		}
		if len(observations) == 1 {
			if observations[0].Type != "file_change" {
				t.Fatalf("event observation type = %q, want file_change", observations[0].Type)
			}
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("controller ended: %v", err)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("controller did not apply exactly one event before timeout")
}

func TestLoadConfigRequiresExplicitProjectPort(t *testing.T) {
	directory := t.TempDir()
	if _, err := LoadConfig("project", directory, t.TempDir()); err == nil {
		t.Fatal("missing config.toml must fail")
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[mnemo.events]\nport = 4222\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig("project", directory, t.TempDir())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Port != 4222 {
		t.Fatalf("configured port = %d, want 4222", cfg.Port)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return port
}
