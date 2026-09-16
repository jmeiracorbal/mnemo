package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const rpcSubject = "mnemo.controller.rpc"

// RPCRequest is the shared controller request envelope. It carries no SQLite
// details; action names are interpreted exclusively by the controller.
type RPCRequest struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type RPCResponse struct {
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Call sends an MCP operation to the installation-wide controller. Clients do
// not retain a Store or a database path.
func Call(ctx context.Context, cfg Config, action string, payload any, output any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode controller request: %w", err)
	}
	request, err := json.Marshal(RPCRequest{Action: action, Payload: encoded})
	if err != nil {
		return fmt.Errorf("encode RPC envelope: %w", err)
	}
	nc, err := nats.Connect(cfg.URL(), nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect global controller: %w", err)
	}
	defer nc.Close()
	if isMutation(action) {
		return callDurableCommand(ctx, nc, action, encoded, output)
	}
	message, err := nc.RequestWithContext(ctx, rpcSubject, request)
	if err != nil {
		return fmt.Errorf("request controller: %w", err)
	}
	var response RPCResponse
	if err := json.Unmarshal(message.Data, &response); err != nil {
		return fmt.Errorf("decode controller response: %w", err)
	}
	if response.Error != "" {
		return fmt.Errorf("controller: %s", response.Error)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(response.Payload, output); err != nil {
		return fmt.Errorf("decode controller payload: %w", err)
	}
	return nil
}

func isMutation(action string) bool {
	switch action {
	case "resolve_session", "close_sessions", "bind_execution_session", "add_observation", "update_observation", "delete_observation", "add_prompt", "passive_capture", "merge_tags", "agent_tool":
		return true
	default:
		return false
	}
}

func callDurableCommand(ctx context.Context, nc *nats.Conn, action string, payload json.RawMessage, output any) error {
	command, err := NewCommand(action, json.RawMessage(payload))
	if err != nil {
		return err
	}
	inbox := nats.NewInbox()
	subscription, err := nc.SubscribeSync(inbox)
	if err != nil {
		return fmt.Errorf("subscribe durable command reply: %w", err)
	}
	defer func() { _ = subscription.Unsubscribe() }()
	command.Reply = inbox
	encoded, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("encode command reply subject: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("open durable command stream: %w", err)
	}
	if _, err := js.PublishMsg(&nats.Msg{Subject: commandSubject, Data: encoded, Header: nats.Header{"Nats-Msg-Id": []string{command.ID}}}, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish durable command: %w", err)
	}
	message, err := subscription.NextMsgWithContext(ctx)
	if err != nil {
		return fmt.Errorf("await durable command: %w", err)
	}
	var response RPCResponse
	if err := json.Unmarshal(message.Data, &response); err != nil {
		return fmt.Errorf("decode command response: %w", err)
	}
	if response.Error != "" {
		return fmt.Errorf("controller: %s", response.Error)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(response.Payload, output); err != nil {
		return fmt.Errorf("decode command payload: %w", err)
	}
	return nil
}
