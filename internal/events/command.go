package events

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Command is the durable controller boundary for MCP mutations. MCP clients
// publish commands; only the controller may apply one to SQLite.
type Command struct {
	ID      string          `json:"id"`
	Action  string          `json:"action"`
	Reply   string          `json:"reply"`
	Payload json.RawMessage `json:"payload"`
}

func NewCommand(action string, payload any) (Command, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Command{}, fmt.Errorf("encode command payload: %w", err)
	}
	command := Command{ID: uuid.NewString(), Action: action, Payload: data}
	return command, command.Validate()
}

func (c Command) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Action) == "" {
		return fmt.Errorf("command id and action are required")
	}
	if !json.Valid(c.Payload) {
		return fmt.Errorf("command payload must be valid JSON")
	}
	return nil
}
