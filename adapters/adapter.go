// Package adapters defines the common, agent-agnostic execution adapter
// contract used by mnemo integrations.
//
// An ExecutionID is not a mnemo session ID. It is a stable correlation key
// shared by hook events and MCP calls; only the controller may map it to an
// internal mnemo session.
package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const domain = "mnemo.execution.v1"

type Agent string

const (
	AgentClaudeCode Agent = "claudecode"
	AgentCodex      Agent = "codex"
	AgentCursor     Agent = "cursor"
	AgentWindsurf   Agent = "windsurf"
	AgentOpenCode   Agent = "opencode"
	AgentPi         Agent = "pi"
)

// Identity is the only cross-agent input to the event controller. NativeID is
// adapter-owned: it must come from the platform event, never a path or PID.
type Identity struct {
	Agent     Agent
	Project   string
	NativeID  string
	Execution string
}

// Capabilities describes what an adapter's platform can provide today. Every
// supported agent has an adapter; a missing platform feature is explicit here,
// never represented by a missing adapter or a guessed identity.
type Capabilities struct {
	NativeExecutionID bool
	JSONHookPayload   bool
}

// Adapter is the common contract for every agent integration. It converts a
// native execution identifier explicitly supplied by that agent into mnemo's
// opaque execution key; it never guesses one.
type Adapter interface {
	Agent() Agent
	Capabilities() Capabilities
	Identity(project, nativeID string) (Identity, error)
}

// HookAdapter is implemented only by integrations whose hook payload is JSON.
// Other integrations may supply their native ID directly through their API.
type HookAdapter interface {
	Adapter
	ParseHook(payload []byte, project string) (Identity, error)
}

type jsonAdapter struct {
	agent Agent
	field string
}

func (a jsonAdapter) Agent() Agent { return a.agent }

func (a jsonAdapter) Capabilities() Capabilities {
	return Capabilities{NativeExecutionID: true, JSONHookPayload: true}
}

func (a jsonAdapter) Identity(project, nativeID string) (Identity, error) {
	return deriveIdentity(a.agent, project, nativeID)
}

func (a jsonAdapter) ParseHook(payload []byte, project string) (Identity, error) {
	var input map[string]json.RawMessage
	if err := json.Unmarshal(payload, &input); err != nil {
		return Identity{}, fmt.Errorf("parse %s hook payload: %w", a.agent, err)
	}
	raw, ok := input[a.field]
	if !ok {
		return Identity{}, fmt.Errorf("%s hook payload requires %q", a.agent, a.field)
	}
	var nativeID string
	if err := json.Unmarshal(raw, &nativeID); err != nil {
		return Identity{}, fmt.Errorf("%s hook payload %q must be a string", a.agent, a.field)
	}
	return a.Identity(project, nativeID)
}

type directAdapter struct{ agent Agent }

func (a directAdapter) Agent() Agent { return a.agent }

func (a directAdapter) Capabilities() Capabilities {
	return Capabilities{NativeExecutionID: true}
}

func (a directAdapter) Identity(project, nativeID string) (Identity, error) {
	return deriveIdentity(a.agent, project, nativeID)
}

var adapters = map[Agent]Adapter{
	AgentClaudeCode: jsonAdapter{agent: AgentClaudeCode, field: "session_id"},
	AgentCodex:      jsonAdapter{agent: AgentCodex, field: "session_id"},
	AgentCursor:     jsonAdapter{agent: AgentCursor, field: "conversation_id"},
	AgentWindsurf:   jsonAdapter{agent: AgentWindsurf, field: "trajectory_id"},
	AgentOpenCode:   directAdapter{agent: AgentOpenCode},
	AgentPi:         directAdapter{agent: AgentPi},
}

// AdapterFor returns an adapter only where the integration has an explicit
// native execution identity contract. Unsupported agents are not approximated.
func AdapterFor(agent Agent) (Adapter, bool) {
	a, ok := adapters[agent]
	return a, ok
}

// HookAdapterFor returns the JSON hook parser, when the agent uses one.
func HookAdapterFor(agent Agent) (HookAdapter, bool) {
	adapter, ok := adapters[agent]
	if !ok {
		return nil, false
	}
	hookAdapter, ok := adapter.(HookAdapter)
	return hookAdapter, ok
}

// Supports reports whether an adapter can currently correlate a native
// execution identity. Unsupported platforms deliberately return false; they
// must not be approximated from a PID, path or recent session.
func Supports(agent Agent) bool {
	adapter, ok := AdapterFor(agent)
	return ok && adapter.Capabilities().NativeExecutionID
}

// NewIdentity creates a deterministic opaque execution key from the canonical
// project ID, adapter identity and native agent execution ID.
func NewIdentity(agent Agent, project, nativeID string) (Identity, error) {
	adapter, ok := AdapterFor(agent)
	if !ok {
		return Identity{}, fmt.Errorf("unknown agent %q", agent)
	}
	return adapter.Identity(project, nativeID)
}

func deriveIdentity(agent Agent, project, nativeID string) (Identity, error) {
	if strings.TrimSpace(project) == "" {
		return Identity{}, errors.New("project id must not be empty")
	}
	if strings.TrimSpace(nativeID) == "" {
		return Identity{}, fmt.Errorf("%s native execution id must not be empty", agent)
	}
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(agent))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(project))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(nativeID))
	return Identity{Agent: agent, Project: project, NativeID: nativeID, Execution: hex.EncodeToString(h.Sum(nil))}, nil
}
