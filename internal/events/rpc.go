package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	goevents "github.com/jmeiracorbal/go-events"
	goeventsnats "github.com/jmeiracorbal/go-events/adapters/nats"
	"github.com/jmeiracorbal/mnemo/v2/protocol"
	"github.com/nats-io/nats.go"
)

const rpcSubject = "mnemo.controller.rpc"

// Call executes a versioned Mnemo request against the sole controller.
// Mutations use JetStream; queries use NATS request/reply. There is no
// controller action-string protocol and no compatibility path.
func Call(ctx context.Context, cfg Config, request protocol.Request, output *protocol.Response) error {
	if err := request.Validate(); err != nil { return err }
	spec, _ := request.Operation.Spec()
	if spec.Kind == protocol.KindEvent {
		return &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "event.publish must use the durable event API"}
	}
	nc, err := nats.Connect(cfg.URL(), nats.Timeout(2*time.Second))
	if err != nil { return fmt.Errorf("connect Mnemo controller: %w", err) }
	defer nc.Close()
	body, err := json.Marshal(request)
	if err != nil { return fmt.Errorf("encode Mnemo request: %w", err) }
	if spec.Kind == protocol.KindCommand {
		return callDurableCommand(ctx, cfg, nc, request, output)
	}
	message, err := nc.RequestWithContext(ctx, rpcSubject, body)
	if err != nil { return fmt.Errorf("request Mnemo controller: %w", err) }
	if err := json.Unmarshal(message.Data, output); err != nil {
		return fmt.Errorf("decode Mnemo response: %w", err)
	}
	return nil
}

func callDurableCommand(ctx context.Context, cfg Config, nc *nats.Conn, request protocol.Request, output *protocol.Response) error {
	command, err := NewCommand(request)
	if err != nil { return err }
	inbox := nats.NewInbox()
	subscription, err := nc.SubscribeSync(inbox)
	if err != nil { return fmt.Errorf("subscribe command reply: %w", err) }
	defer func() { _ = subscription.Unsubscribe() }()
	command.Reply = inbox
	encoded, err := json.Marshal(command)
	if err != nil { return fmt.Errorf("encode durable command: %w", err) }
	adapter, err := goeventsnats.New(ctx, goeventsnats.Config{
		URL: cfg.URL(), Stream: commandStreamName,
	}, nats.Timeout(2*time.Second))
	if err != nil { return fmt.Errorf("open durable command transport: %w", err) }
	defer func() { _ = adapter.Close() }()
	if err := adapter.Publish(ctx, goevents.Event{
		ID: command.ID, Topic: commandSubject, Data: encoded,
	}); err != nil {
		return fmt.Errorf("publish durable command: %w", err)
	}
	message, err := subscription.NextMsgWithContext(ctx)
	if err != nil { return fmt.Errorf("await durable command: %w", err) }
	if err := json.Unmarshal(message.Data, output); err != nil {
		return fmt.Errorf("decode durable command response: %w", err)
	}
	return nil
}
