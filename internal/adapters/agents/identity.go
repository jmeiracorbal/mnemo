package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const domain = "mnemo.execution.v1"

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

// Adapter is the common contract for every agent integration.
type Adapter interface {
	Agent() Agent
	Capabilities() Capabilities
	Identity(project, nativeID string) (Identity, error)
}

// HookAdapter is implemented only by integrations whose hook payload is JSON.
type HookAdapter interface {
	Adapter
	ParseHook(payload []byte, project string) (Identity, error)
}

// ToolCallAdapter extracts an adapter's native execution identifier from MCP
// tools/call metadata.
type ToolCallAdapter interface {
	Adapter
	ParseToolCallMeta(meta map[string]any, project string) (Identity, error)
}

// EnvironmentAdapter declares the process environment variable through which
// an agent supplies its native execution identity to its MCP child process.
type EnvironmentAdapter interface {
	Adapter
	NativeIDEnvironmentVariable() string
}

// SideChannelAdapter reads the native execution identifier from a filesystem
// side-channel written by the agent's hook before each MCP call arrives.
type SideChannelAdapter interface {
	Adapter
	ReadNativeID(project string) (string, error)
}

// CursorSideChannelPath returns the canonical path where the Cursor
// beforeSubmitPrompt hook writes conversation_id for the MCP server to read.
func CursorSideChannelPath(project string) string {
	return filepath.Join(os.TempDir(), "mnemo-cursor-"+project)
}

// OpenCodeSideChannelPath returns the canonical path where the OpenCode
// session.created plugin event writes the session id for the MCP server to read.
func OpenCodeSideChannelPath(project string) string {
	return filepath.Join(os.TempDir(), "mnemo-opencode-"+project)
}

// BaseJSONAdapter provides a reusable base for agents whose hook payloads are
// JSON objects. Agent sub-packages embed this and extend it with their specific
// interface implementations.
type BaseJSONAdapter struct {
	AgentType Agent
	Field     string
}

func (a BaseJSONAdapter) Agent() Agent { return a.AgentType }

func (a BaseJSONAdapter) Capabilities() Capabilities {
	return Capabilities{NativeExecutionID: true, JSONHookPayload: true}
}

func (a BaseJSONAdapter) Identity(project, nativeID string) (Identity, error) {
	return DeriveIdentity(a.AgentType, project, nativeID)
}

func (a BaseJSONAdapter) ParseHook(payload []byte, project string) (Identity, error) {
	var input map[string]json.RawMessage
	if err := json.Unmarshal(payload, &input); err != nil {
		return Identity{}, fmt.Errorf("parse %s hook payload: %w", a.AgentType, err)
	}
	raw, ok := input[a.Field]
	if !ok {
		return Identity{}, fmt.Errorf("%s hook payload requires %q", a.AgentType, a.Field)
	}
	var nativeID string
	if err := json.Unmarshal(raw, &nativeID); err != nil {
		return Identity{}, fmt.Errorf("%s hook payload %q must be a string", a.AgentType, a.Field)
	}
	return a.Identity(project, nativeID)
}

// BaseDirectAdapter provides a reusable base for agents that supply their
// native ID directly through their context API (no JSON hook payload).
type BaseDirectAdapter struct {
	AgentType Agent
}

func (a BaseDirectAdapter) Agent() Agent { return a.AgentType }

func (a BaseDirectAdapter) Capabilities() Capabilities {
	return Capabilities{NativeExecutionID: true}
}

func (a BaseDirectAdapter) Identity(project, nativeID string) (Identity, error) {
	return DeriveIdentity(a.AgentType, project, nativeID)
}

// DeriveIdentity computes the deterministic opaque execution key from the
// project, agent type and native session identifier. Agent sub-packages use
// this when building their concrete Identity implementations.
func DeriveIdentity(agent Agent, project, nativeID string) (Identity, error) {
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
