package store_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/v2/internal/adapters/agents"
	"github.com/jmeiracorbal/mnemo/v2/internal/events"
	"github.com/jmeiracorbal/mnemo/v2/internal/store"
	"github.com/jmeiracorbal/mnemo/v2/protocol"
)

func TestControllerAppliesEventExactlyOnceAfterExecutionBinding(t *testing.T) {
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })

	project := "project-events"
	agent := agents.AgentCodex
	nativeID := "codex-session"
	identity, err := agents.NewIdentity(agent, project, nativeID)
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

	cfg := events.Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := events.NewController(cfg, memory)
	if err != nil {
		t.Fatalf("start controller: %v", err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- controller.Run(ctx) }()

	event, err := events.NewEvent(events.EventWorkspaceFileChanged, project, identity.Execution, map[string]string{"path": "internal/events/events.go", "action": "modified"})
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	if err := events.Publish(context.Background(), cfg, event); err != nil {
		t.Fatalf("publish first event: %v", err)
	}
	if err := events.Publish(context.Background(), cfg, event); err != nil {
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
	cfg := events.Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := events.NewController(cfg, memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = controller.Run(ctx) }()
	event := events.Event{ID: "pi-start", Type: events.EventExecutionStarted, Project: "project-pi", Agent: agents.AgentPi, NativeID: "native-pi-session", Payload: []byte(`{"directory":"/tmp/pi-project"}`), OccurredAt: time.Now().UTC()}
	if err := events.Publish(context.Background(), cfg, event); err != nil {
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
	cfg := events.Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := events.NewController(cfg, memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = controller.Run(ctx) }()

	resolvedRequest, err := protocol.NewRequest(protocol.OperationSessionResolve, "controller-test-session", protocol.SessionResolveInput{
		ProjectID: "project-rpc", Directory: t.TempDir(), InstanceID: "instance-rpc", PID: 7,
	})
	if err != nil { t.Fatal(err) }
	var resolvedResponse protocol.Response
	if err := events.Call(context.Background(), cfg, resolvedRequest, &resolvedResponse); err != nil {
		t.Fatalf("resolve durable Mnemo session: %v", err)
	}
	if resolvedResponse.Error != nil { t.Fatal(resolvedResponse.Error) }
	var resolved protocol.SessionResolveOutput
	if err := json.Unmarshal(resolvedResponse.Payload, &resolved); err != nil { t.Fatal(err) }
	sessionID := resolved.SessionID
	request, err := protocol.NewRequest(protocol.OperationObservationCreate, "controller-test-request", protocol.ObservationCreateInput{
		SessionID: sessionID, ProjectID: "project-rpc", Type: "manual",
		Title: "Through controller", Content: "The MCP process did not open SQLite.",
	})
	if err != nil { t.Fatal(err) }
	var response protocol.Response
	if err := events.Call(context.Background(), cfg, request, &response); err != nil {
		t.Fatalf("save durable public observation: %v", err)
	}
	if response.Error != nil { t.Fatal(response.Error) }
	var created protocol.ObservationCreateOutput
	if err := json.Unmarshal(response.Payload, &created); err != nil { t.Fatal(err) }
	observations, err := memory.SessionObservations(sessionID, 10)
	if err != nil || len(observations) != 1 || observations[0].SyncID != created.Ref.SyncID {
		t.Fatalf("controller did not persist observation in the resolved session: %+v, %v", observations, err)
	}
}

func mustExecution(t *testing.T, event events.Event) string {
	t.Helper()
	identity, err := agents.NewIdentity(event.Agent, event.Project, event.NativeID)
	if err != nil {
		t.Fatal(err)
	}
	return identity.Execution
}

func TestLoadConfigRequiresExplicitGlobalPort(t *testing.T) {
	directory := t.TempDir()
	if _, err := events.LoadConfig(directory); err == nil {
		t.Fatal("missing config.toml must fail")
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[events]\nport = 4222\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := events.LoadConfig(directory)
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

func TestUpgradeVerificationWithRealSQLiteAndDurableStreams(t *testing.T) {
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := memory.Close(); err != nil {
			t.Errorf("close memory store: %v", err)
		}
	})
	cfg := events.Config{Port: freePort(t), DataDir: t.TempDir()}
	controller, err := events.NewController(cfg, memory)
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- controller.Run(ctx) }()
	if err := events.VerifyUpgrade(ctx, cfg); err != nil {
		t.Fatalf("upgrade verification did not prove SQLite and durable processing: %v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	stats, err := memory.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalSessions != 0 || stats.TotalObservations != 0 {
		t.Fatalf("health verification modified user memory state: %+v", stats)
	}
}
