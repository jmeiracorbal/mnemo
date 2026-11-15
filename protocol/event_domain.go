package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Event is a durable Mnemo domain event. It is separate from the mnemo/1
// RPC request envelope and retains the caller's event identity on retries.
type Event struct {
	ID string `json:"id"`
	Type string `json:"type"`
	ProjectID string `json:"project_id"`
	ExecutionKey string `json:"execution_key,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	NativeID string `json:"native_id,omitempty"`
	Payload json.RawMessage `json:"payload"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.Type) == "" || !validResourceToken(e.ProjectID) {
		return &Error{Code: CodeInvalidRequest, Message: "event id, type and Mnemo project_id are required"}
	}
	if e.ExecutionKey == "" && (e.AgentID == "" || e.NativeID == "") {
		return &Error{Code: CodeInvalidRequest, Message: "event execution_key or agent_id and native_id are required"}
	}
	if !json.Valid(e.Payload) || e.OccurredAt.IsZero() {
		return &Error{Code: CodeInvalidRequest, Message: fmt.Sprintf("event payload and occurred_at must be valid")}
	}
	return nil
}
