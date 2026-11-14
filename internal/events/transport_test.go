package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestUpgradeReadinessRejectsPendingMessagesAndAcceptsDrainedDurables(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, &transportTestStore{applied: make(chan DurableEvent, 2)})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	eventSub, err := controller.js.PullSubscribe(cfg.subject(), consumerName, nats.BindStream(streamName))
	if err != nil {
		t.Fatal(err)
	}
	commandSub, err := controller.js.PullSubscribe(commandSubject, commandConsumerName, nats.BindStream(commandStreamName))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := CheckUpgradeReady(ctx, cfg); err != nil {
		t.Fatalf("empty consumers should be safe: %v", err)
	}
	for _, item := range []struct{
		stream string
		subject string
		sub *nats.Subscription
	}{
		{streamName, cfg.publishSubject("old-project"), eventSub},
		{commandStreamName, commandSubject, commandSub},
	} {
		body := []byte(`{"id":"persisted-before-upgrade"}`)
		if _, err := controller.js.Publish(item.subject, body); err != nil {
			t.Fatal(err)
		}
		if err := CheckUpgradeReady(ctx, cfg); err == nil || !strings.Contains(err.Error(), "upgrade blocked") {
			t.Fatalf("expected %s pending to block upgrade: %v", item.stream, err)
		}
		messages, err := item.sub.Fetch(1, nats.MaxWait(2*time.Second))
		if err != nil || len(messages) != 1 {
			t.Fatalf("fetch %s: %v", item.stream, err)
		}
		if err := CheckUpgradeReady(ctx, cfg); err == nil || !strings.Contains(err.Error(), "awaiting acknowledgment") {
			t.Fatalf("expected %s unacked delivery to block upgrade: %v", item.stream, err)
		}
		if err := messages[0].AckSync(); err != nil {
			t.Fatal(err)
		}
		if err := CheckUpgradeReady(ctx, cfg); err != nil {
			t.Fatalf("drained %s should be safe: %v", item.stream, err)
		}
		stored, err := controller.js.GetLastMsg(item.stream, item.subject)
		if err != nil {
			t.Fatal(err)
		}
		if string(stored.Data) != string(body) || stored.Header.Get("Nats-Msg-Id") != "" {
			t.Fatalf("preflight rewrote legacy %s message", item.stream)
		}
	}
}

func TestUpgradeReadinessRejectsHistoryWithoutConsumer(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, &transportTestStore{applied: make(chan DurableEvent, 2)})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	if _, err := controller.js.Publish(cfg.publishSubject("old-project"), []byte(`{"id":"orphan"}`)); err != nil {
		t.Fatal(err)
	}
	if err := CheckUpgradeReady(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "durable consumer is absent") {
		t.Fatalf("expected missing durable with historical messages to block upgrade: %v", err)
	}
}

func TestTransportErrorReporterPropagatesFatalErrorsOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failures := make(chan error, 1)
	report := transportErrorReporter(ctx, failures)
	retryable := fmt.Errorf("handle event: %w", errRetryableDelivery)
	report(retryable)
	select {
	case err := <-failures:
		t.Fatalf("retryable handler failure was fatal: %v", err)
	default:
	}
	fatal := errors.New("durable acknowledgment failed")
	report(fatal)
	select {
	case err := <-failures:
		if !errors.Is(err, fatal) {
			t.Fatalf("reported error = %v, want %v", err, fatal)
		}
	default:
		t.Fatal("fatal transport failure was suppressed")
	}
	cancel()
	report(errors.New("shutdown canceled acknowledgment"))
	select {
	case err := <-failures:
		t.Fatalf("graceful shutdown caused fatal error: %v", err)
	default:
	}
}

func TestUpgradeGraceWaitsForBothDurablesToBeAcknowledged(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, &transportTestStore{applied: make(chan DurableEvent, 2)})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	eventsSub, err := controller.js.PullSubscribe(cfg.subject(), consumerName, nats.BindStream(streamName))
	if err != nil {
		t.Fatal(err)
	}
	commandsSub, err := controller.js.PullSubscribe(commandSubject, commandConsumerName, nats.BindStream(commandStreamName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.js.Publish(cfg.publishSubject("old-project"), []byte(`{"id":"pending-event"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.js.Publish(commandSubject, []byte(`{"id":"pending-command"}`)); err != nil {
		t.Fatal(err)
	}

	acknowledged := make(chan error, 1)
	go func() {
		time.Sleep(250 * time.Millisecond)
		for _, sub := range []*nats.Subscription{eventsSub, commandsSub} {
			messages, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
			if err != nil {
				acknowledged <- err
				return
			}
			if len(messages) != 1 {
				acknowledged <- fmt.Errorf("got %d pending deliveries, want one", len(messages))
				return
			}
			if err := messages[0].AckSync(); err != nil {
				acknowledged <- err
				return
			}
		}
		acknowledged <- nil
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	started := time.Now()
	if err := WaitUpgradeReady(ctx, cfg, 350*time.Millisecond); err != nil {
		t.Fatalf("upgrade should proceed after grace: %v", err)
	}
	if err := <-acknowledged; err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 500*time.Millisecond {
		t.Fatalf("upgrade did not wait for acknowledged saves and quiet window: %s", elapsed)
	}
}

func TestUpgradeGraceDoesNotReplaceControllerWhileSavesArePending(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, &transportTestStore{applied: make(chan DurableEvent, 2)})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	if _, err := controller.js.PullSubscribe(cfg.subject(), consumerName, nats.BindStream(streamName)); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"id":"still-pending"}`)
	published, err := controller.js.Publish(cfg.publishSubject("old-project"), body)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err = WaitUpgradeReady(ctx, cfg, 350*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "pending saves") {
		t.Fatalf("pending saves must block an unsafe cutover: %v", err)
	}
	stored, err := controller.js.GetMsg(streamName, published.Sequence)
	if err != nil || string(stored.Data) != string(body) {
		t.Fatalf("pending event was lost or modified: %v", err)
	}
}

func TestUpgradeGraceRequiresQuietPeriodWhenQueueInitiallyEmpty(t *testing.T) {
	cfg := Config{Port: transportTestFreePort(t), DataDir: t.TempDir()}
	controller, err := NewController(cfg, &transportTestStore{applied: make(chan DurableEvent, 2)})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	if _, err := controller.js.PullSubscribe(cfg.subject(), consumerName, nats.BindStream(streamName)); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.js.PullSubscribe(commandSubject, commandConsumerName, nats.BindStream(commandStreamName)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := time.Now()
	if err := WaitUpgradeReady(ctx, cfg, 350*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 350*time.Millisecond {
		t.Fatalf("empty queue did not receive a grace period: %s", elapsed)
	}
}
