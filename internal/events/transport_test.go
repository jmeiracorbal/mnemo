package events

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type transportTestStore struct {
	applied chan DurableEvent
}

func (s *transportTestStore) ApplyDurableEvent(event DurableEvent) error {
	s.applied <- event
	return nil
}

func (*transportTestStore) ApplyMCPCommand(string, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`null`), nil
}

func (*transportTestStore) ExecuteMCPAction(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`null`), nil
}

func TestPublishedEventRetainsWireShapeAndAddsTransportIdentity(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	store := &transportTestStore{applied: make(chan DurableEvent, 2)}
	controller, err := NewController(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	event, err := NewEvent(EventExecutionStarted, "test-project", "test-execution", map[string]string{"name":"sample"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Publish(context.Background(), cfg, event); err != nil {
		t.Fatal(err)
	}
	message, err := controller.js.GetLastMsg(streamName, cfg.publishSubject(event.Project))
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("Nats-Msg-Id") != event.ID {
		t.Fatalf("transport ID = %q; expected %q", message.Header.Get("Nats-Msg-Id"), event.ID)
	}
	original, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if string(message.Data) != string(original) {
		t.Fatalf("event JSON changed: %s; expected %s", message.Data, original)
	}
}

func TestUnacknowledgedLegacyMessagesBlockCutover(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	store := &transportTestStore{applied: make(chan DurableEvent, 2)}
	controller, err := NewController(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	if _, err := controller.js.PullSubscribe(cfg.subject(), consumerName, nats.BindStream(streamName)); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.js.Publish(cfg.publishSubject("previous"), []byte(`{"id":"legacy-event"}`)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := controller.Run(ctx); err == nil || !strings.Contains(err.Error(), "unsafe go-events cutover") {
		t.Fatalf("expected explicit cutover failure, got %v", err)
	}
}

func TestAcknowledgedLegacyMessagesSurviveRestartWithoutReplay(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	store := &transportTestStore{applied: make(chan DurableEvent, 2)}
	old, err := NewController(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := old.js.PullSubscribe(cfg.subject(), consumerName, nats.BindStream(streamName))
	if err != nil {
		t.Fatal(err)
	}
	legacyData := []byte(`{"id":"old-event","type":"execution.started","project":"previous","execution_key":"old","payload":{},"occurred_at":"2026-11-13T00:00:00Z"}`)
	published, err := old.js.Publish(cfg.publishSubject("previous"), legacyData)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
	if err != nil || len(messages) != 1 {
		t.Fatalf("fetch legacy delivery: %v", err)
	}
	if err := messages[0].AckSync(); err != nil {
		t.Fatal(err)
	}
	old.Close()
	updated, err := NewController(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	defer updated.Close()
	stored, err := updated.js.GetMsg(streamName, published.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored.Data) != string(legacyData) || stored.Header.Get("Nats-Msg-Id") != "" {
		t.Fatal("original durable payload was modified")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- updated.Run(ctx) }()
	event, err := NewEvent(EventExecutionStarted, "new-project", "new-execution", map[string]string{"name":"new"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Publish(context.Background(), cfg, event); err != nil {
		t.Fatal(err)
	}
	select {
	case applied := <-store.applied:
		if applied.ID != event.ID || applied.Project != event.Project {
			t.Fatalf("unexpected replayed event: %+v", applied)
		}
	case <-time.After(5*time.Second):
		t.Fatal("new event not delivered after restart")
	}
	// Processing the store effect is not yet an acknowledgment. Wait for both
	// durable consumers to be ready and the new event to be acknowledged before
	// canceling the controller context.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		eventConsumer, eventErr := updated.js.ConsumerInfo(streamName, consumerName)
		_, commandErr := updated.js.ConsumerInfo(commandStreamName, commandConsumerName)
		if eventErr == nil && commandErr == nil && eventConsumer.NumAckPending == 0 &&
			eventConsumer.AckFloor.Stream > published.Sequence {
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	shutdownErr := <-done
	t.Fatalf("updated controller did not acknowledge delivery and initialize command consumer (shutdown: %v)", shutdownErr)
}

func transportTestFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}
