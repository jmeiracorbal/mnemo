package events

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/adapters"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

func TestControllerAppliesEventExactlyOnceAfterExecutionBinding(t *testing.T) {
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })

	project := "project-events"
	agent := adapters.AgentCodex
	nativeID := "codex-session"
	identity, err := adapters.NewIdentity(agent, project, nativeID)
	if err != nil {
		t.Fatalf("derive execution identity: %v", err)
	}
	sessionID, err := memory.ResolveMCPInstanceSession(project, t.TempDir(), "mcp-instance", 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := memory.BindExecutionSession(project, agent, nativeID, sessionID); err != nil {
		t.Fatalf("bind execution session: %v", err)
	}

	cfg := Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, memory)
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- controller.Run(ctx) }()

	event, err := NewEvent(EventWorkspaceFileChanged, project, identity.Execution, map[string]string{"path": "internal/events/events.go", "action": "modified"})
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

func TestControllerDerivesExecutionKeyFromNativeIdentity(t *testing.T) {
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	cfg := Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = controller.Run(ctx) }()
	event := Event{ID: "pi-start", Type: EventExecutionStarted, Project: "project-pi", Agent: adapters.AgentPi, NativeID: "native-pi-session", Payload: []byte(`{"directory":"/tmp/pi-project"}`), OccurredAt: time.Now().UTC()}
	if err := Publish(context.Background(), cfg, event); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := memory.GetSession("execution-" + mustExecution(t, event)); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("controller did not create execution session")
}

func TestControllerExecutesMCPMutationsThroughDurableCommands(t *testing.T) {
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	cfg := Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = controller.Run(ctx) }()

	var sessionID string
	if err := Call(context.Background(), cfg, "resolve_session", struct {
		Project    string `json:"project"`
		Directory  string `json:"directory"`
		InstanceID string `json:"instance_id"`
		PID        int    `json:"pid"`
	}{"project-rpc", t.TempDir(), "instance-rpc", 7}, &sessionID); err != nil {
		t.Fatalf("resolve durable MCP session: %v", err)
	}
	var observationID int64
	if err := Call(context.Background(), cfg, "add_observation", store.AddObservationParams{SessionID: sessionID, Type: "manual", Title: "Through controller", Content: "The MCP process did not open SQLite.", Project: "project-rpc"}, &observationID); err != nil {
		t.Fatalf("save durable MCP observation: %v", err)
	}
	if observationID == 0 {
		t.Fatal("durable command returned no observation ID")
	}
	if _, err := memory.GetObservation(observationID); err != nil {
		t.Fatalf("controller did not persist observation: %v", err)
	}
}

func mustExecution(t *testing.T, event Event) string {
	t.Helper()
	identity, err := adapters.NewIdentity(event.Agent, event.Project, event.NativeID)
	if err != nil {
		t.Fatal(err)
	}
	return identity.Execution
}

func TestLoadConfigRequiresExplicitGlobalPort(t *testing.T) {
	directory := t.TempDir()
	if _, err := LoadConfig(directory); err == nil {
		t.Fatal("missing config.toml must fail")
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[events]\nport = 4222\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig(directory)
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
