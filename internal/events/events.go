// Package events provides the durable, typed boundary between agent hooks and
// the mnemo controller. Publishers never open SQLite; only Controller does.
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
	"github.com/jmeiracorbal/mnemo/adapters"
	"github.com/jmeiracorbal/mnemo/internal/store"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/pelletier/go-toml/v2"
)

const (
	configFilename = "config.toml"
	streamName     = "MNEMO_EVENTS"
	consumerName   = "mnemo-controller"

	EventExecutionStarted     = store.EventExecutionStarted
	EventExecutionClosed      = store.EventExecutionClosed
	EventSessionCompacted     = store.EventSessionCompacted
	EventWorkspaceFileChanged = store.EventWorkspaceFileChanged
	EventGitCommitCreated     = store.EventGitCommitCreated
)

// Event is the wire contract shared by every agent adapter.
type Event struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Project      string          `json:"project"`
	ExecutionKey string          `json:"execution_key,omitempty"`
	Agent        adapters.Agent  `json:"agent,omitempty"`
	NativeID     string          `json:"native_id,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	OccurredAt   time.Time       `json:"occurred_at"`
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

// EnsureConfig creates the global controller configuration exactly once. The
// listener port is technical infrastructure; users may subsequently change it.
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
	nc, err := nats.Connect(cfg.URL(), nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect event controller: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("open JetStream: %w", err)
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	if _, err := js.PublishMsg(&nats.Msg{Subject: cfg.publishSubject(event.Project), Data: body}, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	return nil
}

// Controller embeds a local NATS JetStream server and is the sole component
// that invokes store.ApplyDurableEvent.
type Controller struct {
	cfg    Config
	store  *store.Store
	server *natsserver.Server
	nc     *nats.Conn
	js     nats.JetStreamContext
}

func NewController(cfg Config, memory *store.Store) (*Controller, error) {
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
	return nil
}

func (c *Controller) Run(ctx context.Context) error {
	sub, err := c.js.PullSubscribe(c.cfg.subject(), consumerName, nats.BindStream(streamName))
	if err != nil {
		return fmt.Errorf("subscribe event stream: %w", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		messages, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			return fmt.Errorf("fetch durable event: %w", err)
		}
		for _, msg := range messages {
			var event Event
			processErr := json.Unmarshal(msg.Data, &event)
			if processErr == nil {
				processErr = event.Validate()
			}
			if processErr == nil && event.ExecutionKey == "" {
				identity, err := adapters.NewIdentity(event.Agent, event.Project, event.NativeID)
				if err != nil {
					processErr = err
				} else {
					event.ExecutionKey = identity.Execution
				}
			}
			if processErr == nil {
				processErr = c.store.ApplyDurableEvent(store.DurableEvent{ID: event.ID, Type: event.Type, Project: event.Project, ExecutionKey: event.ExecutionKey, Payload: event.Payload})
			}
			if processErr != nil {
				// The event remains durable and is retried after the mapping or a
				// transient SQLite failure is repaired. Never ACK before a commit.
				if nakErr := msg.NakWithDelay(time.Second); nakErr != nil {
					return fmt.Errorf("process event: %v; delay redelivery: %w", processErr, nakErr)
				}
				continue
			}
			if err := msg.Ack(); err != nil {
				return fmt.Errorf("ack durable event: %w", err)
			}
		}
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

// LocalAddress is exposed for diagnostics and tests only; event listeners are
// always loopback-bound and are not a remote API.
func (c *Controller) LocalAddress() net.Addr { return c.server.Addr() }

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
