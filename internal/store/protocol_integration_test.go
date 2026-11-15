package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	goevents "github.com/jmeiracorbal/go-events"
	goeventsnats "github.com/jmeiracorbal/go-events/adapters/nats"
	"github.com/jmeiracorbal/mnemo/client"
	"github.com/jmeiracorbal/mnemo/internal/events"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/jmeiracorbal/mnemo/protocol"
	"github.com/nats-io/nats.go"
)

func newProtocolFixture(t *testing.T) (*store.Store, *client.Client, events.Config) {
	t.Helper()
	memory, err := store.New(store.FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	root := t.TempDir()
	port := freePort(t)
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(fmt.Sprintf("[events]\nport = %d\n", port)), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := events.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := events.NewController(cfg, memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- controller.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("controller: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("controller did not stop")
		}
	})
	api, err := client.New(root)
	if err != nil {
		t.Fatal(err)
	}
	readyCtx, stop := context.WithTimeout(context.Background(), 8*time.Second)
	defer stop()
	for {
		if err := api.Health(readyCtx); err == nil {
			break
		}
		select {
		case <-readyCtx.Done():
			t.Fatal("public controller never became ready")
		case <-time.After(30 * time.Millisecond):
		}
	}
	return memory, api, cfg
}

func TestPublicProtocolCreateGetRetryConflictAndProjectIsolation(t *testing.T) {
	memory, api, _ := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	input := protocol.ObservationCreateInput{
		ProjectID: "project-alpha", Type: "instruction",
		Title: "Project plan", Content: "Finish implementation and verify its behavior",
		Tags: []string{"architecture"},
	}
	first, err := api.Create(ctx, "request-77", input)
	if err != nil {
		t.Fatal(err)
	}
	if first.String() == "" || first.ProjectID != input.ProjectID {
		t.Fatalf("unexpected resource reference: %+v", first)
	}
	retried, err := api.Create(ctx, "request-77", input)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried != first {
		t.Fatalf("retry created a different observation: %v vs %v", retried, first)
	}
	observation, err := api.Get(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Content != input.Content || observation.Type != "instruction" {
		t.Fatalf("unexpected observation: %+v", observation)
	}
	input.Content = "different mutation using the same request id"
	_, err = api.Create(ctx, "request-77", input)
	var coded *protocol.Error
	if !errors.As(err, &coded) || coded.Code != protocol.CodeConflict {
		t.Fatalf("request ID collision error = %v, want conflict", err)
	}
	observations, err := memory.SessionObservations("mnemo-protocol-project-alpha", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 {
		t.Fatalf("retry created %d observations, want one", len(observations))
	}
	_, err = api.Get(ctx, protocol.ResourceRef{ProjectID: "project-beta", SyncID: first.SyncID})
	if !errors.As(err, &coded) || coded.Code != protocol.CodeProjectMismatch {
		t.Fatalf("cross-project lookup error = %v, want project_mismatch", err)
	}
	_, err = api.Get(ctx, protocol.ResourceRef{ProjectID: "project-alpha", SyncID: "obs-missing"})
	if !errors.As(err, &coded) || coded.Code != protocol.CodeNotFound {
		t.Fatalf("missing lookup error = %v, want not_found", err)
	}
}

// Publish a command with no reply subscriber, wait for its SQLite commit,
// then retry through the public client. This exercises a genuinely lost reply
// while the second JetStream delivery uses a distinct transport message ID.
func TestPublicProtocolLostReplyCannotDuplicateCommittedMemory(t *testing.T) {
	memory, api, cfg := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	input := protocol.ObservationCreateInput{
		ProjectID: "project-lost-reply", Type: "instruction",
		Title: "Retained memory", Content: "A committed observation must survive a lost reply",
	}
	request, err := protocol.NewRequest(protocol.OperationObservationCreate, "request-lost-reply", input)
	if err != nil {
		t.Fatal(err)
	}
	command, err := events.NewCommand(protocol.Version, request)
	if err != nil {
		t.Fatal(err)
	}
	command.Reply = nats.NewInbox() // deliberately no subscriber
	encoded, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	bus, err := goeventsnats.New(ctx, goeventsnats.Config{URL: cfg.URL(), Stream: "MNEMO_COMMANDS"}, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bus.Close() }()
	if err := bus.Publish(ctx, goevents.Event{ID: command.ID, Topic: "mnemo.commands", Data: encoded}); err != nil {
		t.Fatal(err)
	}
	sessionID := "mnemo-protocol-" + input.ProjectID
	for {
		observations, err := memory.SessionObservations(sessionID, 10)
		if err == nil && len(observations) == 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("original command did not commit: %v", ctx.Err())
		case <-time.After(30 * time.Millisecond):
		}
	}
	recovered, err := api.Create(ctx, request.RequestID, input)
	if err != nil {
		t.Fatalf("retry after missing response: %v", err)
	}
	observations, err := memory.SessionObservations(sessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 || observations[0].SyncID != recovered.SyncID {
		t.Fatalf("retry replayed mutation or changed identity: %+v, ref=%s", observations, recovered.String())
	}
}

func TestPublicProtocolPreservesActualSessionAndProvenance(t *testing.T) {
	memory, api, _ := newProtocolFixture(t)
	project := "project-agent-session"
	sessionID, err := memory.ResolveMCPInstanceSession(project, t.TempDir(), "mcp-test", 1)
	if err != nil {
		t.Fatal(err)
	}
	input := protocol.ObservationCreateInput{
		ProjectID: project, SessionID: sessionID,
		Type: "decision", Title: "Keep provenance",
		Content: "The session and source of a memory must survive public creation.",
		Provenance: protocol.Provenance{
			AgentID: store.AgentCodex, SourceKindID: store.SourceMCP,
			ToolID: store.ToolMemSave, MCPClientID: store.AgentCodex,
			MCPTransport: "stdio",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ref, err := api.Create(ctx, "mcp-session-write", input)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ProjectID != project {
		t.Fatalf("observation stored under incorrect project: %s", ref.String())
	}
	rows, err := memory.SessionObservations(sessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SyncID != ref.SyncID {
		t.Fatalf("MCP session attribution lost: %+v", rows)
	}
	if rows[0].Provenance == nil || rows[0].Provenance.AgentID != store.AgentCodex ||
		rows[0].Provenance.SourceKindID != store.SourceMCP ||
		rows[0].Provenance.ToolID != store.ToolMemSave {
		t.Fatalf("MCP provenance lost: %+v", rows[0].Provenance)
	}
	retrieved, err := api.Get(ctx, ref)
	if err != nil || retrieved.SessionID != sessionID {
		t.Fatalf("public get changed memory session: %+v, %v", retrieved, err)
	}

	wrongSession, err := memory.ResolveMCPInstanceSession("other-project", t.TempDir(), "other-mcp", 1)
	if err != nil {
		t.Fatal(err)
	}
	input.SessionID = wrongSession
	if _, err := api.Create(ctx, "cross-project-write", input); err == nil {
		t.Fatal("cross-project session accepted")
	}
}
