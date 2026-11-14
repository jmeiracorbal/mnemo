package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	goevents "github.com/jmeiracorbal/go-events"
	goeventsnats "github.com/jmeiracorbal/go-events/adapters/nats"
	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/pelletier/go-toml/v2"
)

const (
	configFilename      = "config.toml"
	streamName          = "MNEMO_EVENTS"
	consumerName        = "mnemo-controller"
	commandStreamName   = "MNEMO_COMMANDS"
	commandConsumerName = "mnemo-controller-commands"
	commandSubject      = "mnemo.commands"

	EventExecutionStarted     = "execution.started"
	EventExecutionClosed      = "execution.closed"
	EventSessionCompacted     = "session.compacted"
	EventAgentToolResult      = "agent.tool_result"
	EventWorkspaceFileChanged = "workspace.file_changed"
	EventGitCommitCreated     = "git.commit_created"
)

// DurableEvent is the controller input persisted by the event broker. EventID
// is globally unique and is the idempotency key; no delivery is acknowledged
// until ApplyDurableEvent commits it with its effect.
type DurableEvent struct {
	ID           string
	Type         string
	Project      string
	ExecutionKey string
	Payload      json.RawMessage
}

// ControllerStore is the only store surface the Controller requires. The
// concrete implementation lives in internal/store; this interface breaks the
// import dependency so events does not couple to the database layer.
type ControllerStore interface {
	ApplyDurableEvent(DurableEvent) error
	ApplyMCPCommand(id, action string, payload json.RawMessage) (json.RawMessage, error)
	ExecuteMCPAction(ctx context.Context, action string, payload json.RawMessage) (json.RawMessage, error)
}

// Event is the wire contract shared by every agent adapter.
type Event struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Project      string       `json:"project"`
	ExecutionKey string       `json:"execution_key,omitempty"`
	Agent        agents.Agent `json:"agent,omitempty"`
	NativeID     string       `json:"native_id,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	OccurredAt   time.Time    `json:"occurred_at"`
}

func NewEvent(eventType, project, executionKey string, payload any) (Event, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	event := Event{ID: uuid.NewString(), Type: eventType, Project: project, ExecutionKey: executionKey, Payload: encoded, OccurredAt: time.Now().UTC()}
	return event, event.Validate()
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.Type) == "" {
		return fmt.Errorf("event id and type are required")
	}
	if strings.TrimSpace(e.Project) == "" {
		return fmt.Errorf("event project is required")
	}
	if strings.TrimSpace(e.ExecutionKey) == "" && (strings.TrimSpace(string(e.Agent)) == "" || strings.TrimSpace(e.NativeID) == "") {
		return fmt.Errorf("event execution key or agent and native id are required")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("event payload must be valid JSON")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("event occurred_at is required")
	}
	return nil
}

type tomlGlobalConfig struct {
	Events struct {
		Port int `toml:"port"`
	} `toml:"events"`
}

// Config is global because mnemo's SQLite store is global; one controller owns
// every project's durable stream.
type Config struct {
	Port    int
	DataDir string
}

// EnsureConfig creates the global controller configuration exactly once.
func EnsureConfig(dataDir string) (string, error) {
	if !filepath.IsAbs(dataDir) {
		return "", fmt.Errorf("event data directory must be absolute")
	}
	path := filepath.Join(dataDir, configFilename)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte("[events]\nport = 4222\n"), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func LoadConfig(dataDir string) (Config, error) {
	if !filepath.IsAbs(dataDir) {
		return Config{}, fmt.Errorf("event data directory must be absolute")
	}
	content, err := os.ReadFile(filepath.Join(dataDir, configFilename))
	if err != nil {
		return Config{}, fmt.Errorf("read global config.toml: %w", err)
	}
	var file tomlGlobalConfig
	if err := toml.Unmarshal(content, &file); err != nil {
		return Config{}, fmt.Errorf("parse global config.toml: %w", err)
	}
	if file.Events.Port < 1 || file.Events.Port > 65535 {
		return Config{}, fmt.Errorf("config.toml [events].port must be between 1 and 65535")
	}
	return Config{Port: file.Events.Port, DataDir: filepath.Join(dataDir, "events")}, nil
}

func (c Config) URL() string                          { return fmt.Sprintf("nats://127.0.0.1:%d", c.Port) }
func (c Config) subject() string                      { return "mnemo.events.>" }
func (c Config) publishSubject(project string) string { return "mnemo.events." + project }

// Publish durably appends an event. A missing controller is an explicit error:
// it is never converted into an in-memory or SQLite fallback.
func Publish(ctx context.Context, cfg Config, event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	adapter, err := goeventsnats.New(ctx, goeventsnats.Config{
		URL: cfg.URL(), Stream: streamName,
	}, nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect event controller: %w", err)
	}
	defer func() { _ = adapter.Close() }()
	if err := adapter.Publish(ctx, goevents.Event{
		ID: event.ID, Topic: cfg.publishSubject(event.Project), Data: body,
	}); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	return nil
}

// Controller embeds a local NATS JetStream server and is the sole component
// that invokes ControllerStore.ApplyDurableEvent.
type Controller struct {
	cfg    Config
	store  ControllerStore
	server *natsserver.Server
	nc     *nats.Conn
	js     nats.JetStreamContext
}

func NewController(cfg Config, memory ControllerStore) (*Controller, error) {
	if memory == nil {
		return nil, fmt.Errorf("event controller store is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("create JetStream storage: %w", err)
	}
	srv, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: cfg.Port, JetStream: true, StoreDir: cfg.DataDir, NoLog: true, NoSigs: true})
	if err != nil {
		return nil, fmt.Errorf("create embedded NATS server: %w", err)
	}
	srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		srv.Shutdown()
		return nil, fmt.Errorf("embedded NATS server did not become ready")
	}
	nc, err := nats.Connect(cfg.URL(), nats.Timeout(2*time.Second))
	if err != nil {
		srv.Shutdown()
		return nil, fmt.Errorf("connect embedded NATS server: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		srv.Shutdown()
		return nil, fmt.Errorf("open embedded JetStream: %w", err)
	}
	controller := &Controller{cfg: cfg, store: memory, server: srv, nc: nc, js: js}
	if err := controller.ensureStream(); err != nil {
		controller.Close()
		return nil, err
	}
	return controller, nil
}

func (c *Controller) ensureStream() error {
	_, err := c.js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{c.cfg.subject()}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return fmt.Errorf("create event stream: %w", err)
	}
	_, err = c.js.AddStream(&nats.StreamConfig{Name: commandStreamName, Subjects: []string{commandSubject}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return fmt.Errorf("create command stream: %w", err)
	}
	return nil
}

// preflightDurables prevents handing an unacknowledged, headerless legacy
// delivery to go-events. It never rewrites or acknowledges stored messages.
func (c *Controller) preflightDurables() error {
	for _, durable := range []struct {
		stream string
		consumer string
	}{
		{streamName, consumerName},
		{commandStreamName, commandConsumerName},
	} {
		stream, err := c.js.StreamInfo(durable.stream)
		if err != nil {
			return fmt.Errorf("inspect stream %s: %w", durable.stream, err)
		}
		if stream.State.Msgs == 0 {
			continue
		}
		consumer, err := c.js.ConsumerInfo(durable.stream, durable.consumer)
		from := stream.State.FirstSeq
		if errors.Is(err, nats.ErrConsumerNotFound) {
			// A first-time installation may have published valid, header-identified
			// events before the consumer was created. Inspect rather than reject them.
		} else if err != nil {
			return fmt.Errorf("inspect durable %s: %w", durable.consumer, err)
		} else {
			if consumer.NumPending == 0 && consumer.NumAckPending == 0 {
				continue
			}
			from = consumer.AckFloor.Stream + 1
		}
		for seq := from; seq <= stream.State.LastSeq; seq++ {
			message, err := c.js.GetMsg(durable.stream, seq)
			if errors.Is(err, nats.ErrMsgNotFound) {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect pending message in %s: %w", durable.stream, err)
			}
			if message.Header.Get("Nats-Msg-Id") == "" {
				return fmt.Errorf("unsafe go-events cutover: stream %s has unacknowledged events without Nats-Msg-Id; drain them using the previous controller before upgrading", durable.stream)
			}
		}
	}
	return nil
}

func (c *Controller) Run(ctx context.Context) error {
	if err := c.preflightDurables(); err != nil {
		return err
	}
	if _, err := c.nc.Subscribe(rpcSubject, c.handleRPC); err != nil {
		return fmt.Errorf("subscribe controller RPC: %w", err)
	}
	if err := c.nc.Flush(); err != nil {
		return fmt.Errorf("flush controller RPC subscription: %w", err)
	}
	transportErrors := make(chan error, 1)
	onTransportError := transportErrorReporter(ctx, transportErrors)
	eventBus, err := goeventsnats.New(ctx, goeventsnats.Config{
		URL: c.cfg.URL(), Stream: streamName, Consumer: consumerName,
		AckWait: 30*time.Second, RetryDelay: time.Second, OnError: onTransportError,
	}, nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("open event transport: %w", err)
	}
	defer func() { _ = eventBus.Close() }()
	commandBus, err := goeventsnats.New(ctx, goeventsnats.Config{
		URL: c.cfg.URL(), Stream: commandStreamName, Consumer: commandConsumerName,
		AckWait: 30*time.Second, RetryDelay: time.Second, OnError: onTransportError,
	}, nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("open command transport: %w", err)
	}
	defer func() { _ = commandBus.Close() }()
	eventSub, err := eventBus.Subscribe(ctx, c.cfg.subject(), func(ctx context.Context, event goevents.Event) error {
		if err := c.handleEvent(ctx, event); err != nil {
			return fmt.Errorf("%w: %v", errRetryableDelivery, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe event stream: %w", err)
	}
	defer func() { _ = eventSub.Close() }()
	commandSub, err := commandBus.Subscribe(ctx, commandSubject, c.handleCommand)
	if err != nil {
		return fmt.Errorf("subscribe command stream: %w", err)
	}
	defer func() { _ = commandSub.Close() }()
	select {
	case err := <-transportErrors:
		return fmt.Errorf("durable transport failure: %w", err)
	case <-ctx.Done():
		return nil
	}
}

var errRetryableDelivery = errors.New("retryable delivery failure")

// transportErrorReporter preserves handler redelivery but makes failures in
// JetStream itself observable to the controller's lifecycle.
func transportErrorReporter(ctx context.Context, failures chan<- error) func(error) {
	return func(err error) {
		if ctx.Err() != nil || errors.Is(err, errRetryableDelivery) {
			return
		}
		select {
		case failures <- err:
		default:
		}
	}
}

func (c *Controller) handleEvent(_ context.Context, delivery goevents.Event) error {
	var event Event
	if err := json.Unmarshal(delivery.Data, &event); err != nil {
		return fmt.Errorf("decode durable event: %w", err)
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if event.ID != delivery.ID {
		return fmt.Errorf("event ID %q differs from transport ID %q", event.ID, delivery.ID)
	}
	if event.ExecutionKey == "" {
		identity, err := agents.NewIdentity(event.Agent, event.Project, event.NativeID)
		if err != nil {
			return err
		}
		event.ExecutionKey = identity.Execution
	}
	return c.store.ApplyDurableEvent(DurableEvent{
		ID: event.ID, Type: event.Type, Project: event.Project,
		ExecutionKey: event.ExecutionKey, Payload: event.Payload,
	})
}

func (c *Controller) handleCommand(_ context.Context, delivery goevents.Event) error {
	var command Command
	if err := json.Unmarshal(delivery.Data, &command); err != nil {
		// The previous controller acknowledged malformed durable commands
		// without replaying them. Keep the same poison-message behavior.
		return nil
	}
	if err := command.Validate(); err != nil {
		c.replyTo(command.Reply, RPCResponse{Error: err.Error()})
		return nil
	}
	if command.ID != delivery.ID {
		c.replyTo(command.Reply, RPCResponse{Error: "command ID differs from transport ID"})
		return nil
	}
	payload, err := c.store.ApplyMCPCommand(command.ID, command.Action, command.Payload)
	if err != nil {
		c.replyTo(command.Reply, RPCResponse{Error: err.Error()})
		return nil
	}
	c.replyTo(command.Reply, RPCResponse{Payload: payload})
	return nil
}

func (c *Controller) handleRPC(message *nats.Msg) {
	var request RPCRequest
	if err := json.Unmarshal(message.Data, &request); err != nil {
		c.replyRPC(message, RPCResponse{Error: "decode request: " + err.Error()})
		return
	}
	if strings.TrimSpace(request.Action) == "" {
		c.replyRPC(message, RPCResponse{Error: "action is required"})
		return
	}
	payload, err := c.store.ExecuteMCPAction(context.Background(), request.Action, request.Payload)
	if err != nil {
		c.replyRPC(message, RPCResponse{Error: err.Error()})
		return
	}
	c.replyRPC(message, RPCResponse{Payload: payload})
}

func (c *Controller) replyRPC(request *nats.Msg, response RPCResponse) {
	c.replyTo(request.Reply, response)
}

func (c *Controller) replyTo(subject string, response RPCResponse) {
	if subject == "" {
		return
	}
	payload, err := json.Marshal(response)
	if err == nil {
		_ = c.nc.Publish(subject, payload)
	}
}

func (c *Controller) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
	if c.server != nil {
		c.server.Shutdown()
		c.server.WaitForShutdown()
	}
}

// LocalAddress is exposed for diagnostics and tests only.
func (c *Controller) LocalAddress() net.Addr { return c.server.Addr() }


// CheckUpgradeReady inspects the running, pre-upgrade controller without
// altering stream, consumer, or message state. The installer calls it before
// replacing the binary. Both event and command streams must be fully drained.
func CheckUpgradeReady(ctx context.Context, cfg Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	nc, err := nats.Connect(cfg.URL(), nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect existing controller before upgrade: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream(nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("open existing JetStream before upgrade: %w", err)
	}
	for _, durable := range []struct {
		stream string
		consumer string
		subject string
	}{
		{streamName, consumerName, cfg.subject()},
		{commandStreamName, commandConsumerName, commandSubject},
	} {
		stream, err := js.StreamInfo(durable.stream, nats.Context(ctx))
		if err != nil {
			return fmt.Errorf("inspect existing stream %s: %w", durable.stream, err)
		}
		consumer, err := js.ConsumerInfo(durable.stream, durable.consumer, nats.Context(ctx))
		if errors.Is(err, nats.ErrConsumerNotFound) {
			if stream.State.Msgs != 0 {
				return fmt.Errorf("upgrade blocked: %s contains %d messages but its durable consumer is absent", durable.stream, stream.State.Msgs)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect existing durable %s: %w", durable.consumer, err)
		}
		if consumer.Config.FilterSubject != durable.subject || consumer.Config.AckPolicy != nats.AckExplicitPolicy || consumer.Config.DeliverSubject != "" {
			return fmt.Errorf("upgrade blocked: durable %s has an incompatible configuration", durable.consumer)
		}
		if consumer.NumPending != 0 || consumer.NumAckPending != 0 {
			return fmt.Errorf("upgrade blocked: durable %s has %d pending and %d awaiting acknowledgment; allow the current controller to finish processing", durable.consumer, consumer.NumPending, consumer.NumAckPending)
		}
	}
	return nil
}

// CheckHealth verifies that the singleton controller configured for this
// installation is reachable. MCP processes must call this; they never create it.
func CheckHealth(ctx context.Context, cfg Config) error {
	nc, err := nats.Connect(cfg.URL(), nats.Timeout(time.Second))
	if err != nil {
		return fmt.Errorf("connect global event controller: %w", err)
	}
	defer nc.Close()
	if _, err := nc.JetStream(nats.Context(ctx)); err != nil {
		return fmt.Errorf("open global event controller: %w", err)
	}
	return nil
}
