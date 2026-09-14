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
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Project      string          `json:"project"`
	ExecutionKey string          `json:"execution_key"`
	Payload      json.RawMessage `json:"payload"`
}

func NewCommand(commandType, project, executionKey string, payload any) (Command, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Command{}, fmt.Errorf("encode command payload: %w", err)
	}
	command := Command{ID: uuid.NewString(), Type: commandType, Project: project, ExecutionKey: executionKey, Payload: data}
	return command, command.Validate()
}

func (c Command) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Type) == "" {
		return fmt.Errorf("command id and type are required")
	}
	if strings.TrimSpace(c.Project) == "" || strings.TrimSpace(c.ExecutionKey) == "" {
		return fmt.Errorf("command project and execution key are required")
	}
	if !json.Valid(c.Payload) {
		return fmt.Errorf("command payload must be valid JSON")
	}
	return nil
}
