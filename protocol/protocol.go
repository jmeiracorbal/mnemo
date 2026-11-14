// Package protocol defines Mnemo's stable public memory contract.
//
// Version "mnemo/1" names the domain protocol and its incompatible major
// revision. It is unrelated to release versions, the "mnemo://" resource
// scheme, and the transport used by the controller.
package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Version is the first public Mnemo protocol version. Compatible additions
// keep this identifier; breaking changes require a new major version.
const Version = "mnemo/1"

// Operation names belong to the public Mnemo domain and must not be confused
// with controller-internal action strings.
type Operation string

const (
	OperationHealth            Operation = "health"
	OperationObservationGet    Operation = "observation.get"
	OperationObservationCreate Operation = "observation.create"
)

// Request is a transport-independent protocol envelope. RequestID is assigned
// once by the caller for each logical mutation and MUST be reused for all
// retries, including after a lost reply or timeout. A different mutation must
// use a different ID. The controller must reject reusing an ID with different
// content rather than returning the result of an unrelated mutation.
type Request struct {
	Version   string          `json:"version"`
	Operation Operation       `json:"operation"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewRequest builds the public envelope; it deliberately does not generate a
// new request ID on every send or retry.
func NewRequest(operation Operation, requestID string, payload any) (Request, error) {
	request := Request{Version: Version, Operation: operation, RequestID: requestID}
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return Request{}, &Error{Code: CodeInvalidRequest, Message: fmt.Sprintf("encode payload: %v", err)}
		}
		request.Payload = encoded
	}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}

func (r Request) Validate() error {
	if r.Version != Version {
		return &Error{Code: CodeUnsupportedVersion, Message: fmt.Sprintf("unsupported Mnemo protocol version %q", r.Version)}
	}
	switch r.Operation {
	case OperationHealth:
	case OperationObservationGet:
	case OperationObservationCreate:
		if strings.TrimSpace(r.RequestID) == "" {
			return &Error{Code: CodeInvalidRequest, Message: "observation.create requires a stable request_id"}
		}
	default:
		return &Error{Code: CodeInvalidRequest, Message: fmt.Sprintf("unsupported operation %q", r.Operation)}
	}
	if len(r.Payload) > 0 && !json.Valid(r.Payload) {
		return &Error{Code: CodeInvalidRequest, Message: "payload must contain valid JSON"}
	}
	if r.Operation != OperationHealth && (len(r.Payload) == 0 || string(r.Payload) == "null") {
		return &Error{Code: CodeInvalidRequest, Message: "operation requires a payload"}
	}
	return nil
}

// Response does not contain internal action names or SQLite identifiers.
type Response struct {
	Version   string          `json:"version"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     *Error          `json:"error,omitempty"`
}

// ErrorCode is a stable, machine-readable error category.
type ErrorCode string

const (
	CodeInvalidRequest     ErrorCode = "invalid_request"
	CodeInvalidReference   ErrorCode = "invalid_reference"
	CodeUnsupportedVersion ErrorCode = "unsupported_version"
	CodeNotFound           ErrorCode = "not_found"
	CodeProjectMismatch    ErrorCode = "project_mismatch"
	CodeConflict           ErrorCode = "conflict"
	CodeUnavailable        ErrorCode = "unavailable"
	CodeInternal           ErrorCode = "internal"
)

// Error represents a public protocol failure, without leaking store details.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return string(e.Code) + ": " + e.Message
}

// HealthOutput reports the controller's readiness without exposing transport.
type HealthOutput struct {
	Ready bool `json:"ready"`
}

// ObservationGetInput uses a stable resource reference, never a local row ID.
type ObservationGetInput struct {
	Ref ResourceRef `json:"ref"`
}

// Observation is the public view of a stored Mnemo memory.
type Observation struct {
	Ref       ResourceRef `json:"ref"`
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	Scope     string      `json:"scope,omitempty"`
	TopicKey  string      `json:"topic_key,omitempty"`
	Tags      []string    `json:"tags,omitempty"`
	CreatedAt string      `json:"created_at,omitempty"`
	UpdatedAt string      `json:"updated_at,omitempty"`
}

type ObservationGetOutput struct {
	Observation Observation `json:"observation"`
}

// ObservationCreateInput intentionally does not require SQLite IDs or MCP
// sessions. The controller owns persistence and session attribution.
type ObservationCreateInput struct {
	ProjectID string   `json:"project_id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Scope     string   `json:"scope,omitempty"`
	TopicKey  string   `json:"topic_key,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

func (i ObservationCreateInput) Validate() error {
	if !validResourceToken(i.ProjectID) {
		return &Error{Code: CodeInvalidRequest, Message: "invalid project_id"}
	}
	if strings.TrimSpace(i.Type) == "" || strings.TrimSpace(i.Title) == "" || strings.TrimSpace(i.Content) == "" {
		return &Error{Code: CodeInvalidRequest, Message: "type, title and content are required"}
	}
	return nil
}

type ObservationCreateOutput struct {
	Ref ResourceRef `json:"ref"`
}
