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
	"os"
	"path/filepath"
	"strings"
)

const domain = "mnemo.execution.v1"

type Agent string

const (
	AgentClaudeCode Agent = "claudecode"
	AgentCodex      Agent = "codex"
	AgentCursor     Agent = "cursor"
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

// ToolCallAdapter extracts an adapter's native execution identifier from MCP
// tools/call metadata. It never uses a transport or runtime identifier as a
// substitute when the client omits the documented field.
type ToolCallAdapter interface {
	Adapter
	ParseToolCallMeta(meta map[string]any, project string) (Identity, error)
}

// EnvironmentAdapter declares the process environment variable through which
// an agent supplies its native execution identity to its MCP child process.
// The runtime must reject a missing value rather than substitute an instance,
// path, PID, or recent session.
type EnvironmentAdapter interface {
	Adapter
	NativeIDEnvironmentVariable() string
}

// SideChannelAdapter reads the native execution identifier from a filesystem
// side-channel written by the agent's hook before each MCP call arrives. It is
// used when the agent provides no env var and no tools/call metadata carrier.
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

type claudeCodeAdapter struct{ jsonAdapter }

func (claudeCodeAdapter) NativeIDEnvironmentVariable() string {
	return "CLAUDE_CODE_SESSION_ID"
}

type cursorSideChannelAdapter struct{ jsonAdapter }

func (a cursorSideChannelAdapter) ReadNativeID(project string) (string, error) {
	data, err := os.ReadFile(CursorSideChannelPath(project))
	if err != nil {
		return "", fmt.Errorf("cursor side-channel: %w", err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", errors.New("cursor side-channel: empty conversation_id")
	}
	return id, nil
}

type openCodeSideChannelAdapter struct{ directAdapter }

func (a openCodeSideChannelAdapter) ReadNativeID(project string) (string, error) {
	data, err := os.ReadFile(OpenCodeSideChannelPath(project))
	if err != nil {
		return "", fmt.Errorf("opencode side-channel: %w", err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", errors.New("opencode side-channel: empty session_id")
	}
	return id, nil
}

type codexAdapter struct{ jsonAdapter }

func (a codexAdapter) ParseToolCallMeta(meta map[string]any, project string) (Identity, error) {
	raw, ok := meta["x-codex-turn-metadata"]
	if !ok {
		return Identity{}, errors.New("codex tools/call metadata requires x-codex-turn-metadata")
	}
	turn, ok := raw.(map[string]any)
	if !ok {
		return Identity{}, errors.New("codex tools/call x-codex-turn-metadata must be an object")
	}
	nativeID, ok := turn["session_id"].(string)
	if !ok || strings.TrimSpace(nativeID) == "" {
		return Identity{}, errors.New("codex tools/call metadata requires session_id")
	}
	return a.Identity(project, nativeID)
}

var adapters = map[Agent]Adapter{
	AgentClaudeCode: claudeCodeAdapter{jsonAdapter: jsonAdapter{agent: AgentClaudeCode, field: "session_id"}},
	AgentCodex:      codexAdapter{jsonAdapter: jsonAdapter{agent: AgentCodex, field: "session_id"}},
	AgentCursor:     cursorSideChannelAdapter{jsonAdapter{agent: AgentCursor, field: "conversation_id"}},
	AgentOpenCode: openCodeSideChannelAdapter{directAdapter{agent: AgentOpenCode}},
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

// ToolCallAdapterFor returns the MCP metadata parser when an agent documents a
// native execution identifier in every tools/call request.
func ToolCallAdapterFor(agent Agent) (ToolCallAdapter, bool) {
	adapter, ok := adapters[agent]
	if !ok {
		return nil, false
	}
	toolCallAdapter, ok := adapter.(ToolCallAdapter)
	return toolCallAdapter, ok
}

// EnvironmentAdapterFor returns the adapter that declares a native execution
// identity carrier inherited by the MCP child process, if the agent has one.
func EnvironmentAdapterFor(agent Agent) (EnvironmentAdapter, bool) {
	adapter, ok := adapters[agent]
	if !ok {
		return nil, false
	}
	environmentAdapter, ok := adapter.(EnvironmentAdapter)
	return environmentAdapter, ok
}

// SideChannelAdapterFor returns the filesystem side-channel reader when an
// agent delivers its native execution identifier via a hook-written file.
func SideChannelAdapterFor(agent Agent) (SideChannelAdapter, bool) {
	adapter, ok := adapters[agent]
	if !ok {
		return nil, false
	}
	sc, ok := adapter.(SideChannelAdapter)
	return sc, ok
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
