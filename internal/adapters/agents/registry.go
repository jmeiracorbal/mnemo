package agents

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[Agent]Adapter{}
)

// RegisterAdapter adds or replaces the adapter for one agent. External
// integrators call this from their own init() to register a new agent.
func RegisterAdapter(agent Agent, a Adapter) {
	mu.Lock()
	defer mu.Unlock()
	registry[agent] = a
}

// AdapterFor returns the registered adapter for one agent.
func AdapterFor(agent Agent) (Adapter, bool) {
	mu.RLock()
	defer mu.RUnlock()
	a, ok := registry[agent]
	return a, ok
}

// HookAdapterFor returns the JSON hook parser, when the agent uses one.
func HookAdapterFor(agent Agent) (HookAdapter, bool) {
	a, ok := AdapterFor(agent)
	if !ok {
		return nil, false
	}
	h, ok := a.(HookAdapter)
	return h, ok
}

// ToolCallAdapterFor returns the MCP metadata parser when an agent documents
// a native execution identifier in every tools/call request.
func ToolCallAdapterFor(agent Agent) (ToolCallAdapter, bool) {
	a, ok := AdapterFor(agent)
	if !ok {
		return nil, false
	}
	t, ok := a.(ToolCallAdapter)
	return t, ok
}

// EnvironmentAdapterFor returns the adapter that declares a native execution
// identity carrier inherited by the MCP child process, if the agent has one.
func EnvironmentAdapterFor(agent Agent) (EnvironmentAdapter, bool) {
	a, ok := AdapterFor(agent)
	if !ok {
		return nil, false
	}
	e, ok := a.(EnvironmentAdapter)
	return e, ok
}

// SideChannelAdapterFor returns the filesystem side-channel reader when an
// agent delivers its native execution identifier via a hook-written file.
func SideChannelAdapterFor(agent Agent) (SideChannelAdapter, bool) {
	a, ok := AdapterFor(agent)
	if !ok {
		return nil, false
	}
	sc, ok := a.(SideChannelAdapter)
	return sc, ok
}

// Supports reports whether an adapter can currently correlate a native
// execution identity.
func Supports(agent Agent) bool {
	a, ok := AdapterFor(agent)
	return ok && a.Capabilities().NativeExecutionID
}

// NewIdentity creates a deterministic opaque execution key from the canonical
// project ID, adapter identity and native agent execution ID.
func NewIdentity(agent Agent, project, nativeID string) (Identity, error) {
	a, ok := AdapterFor(agent)
	if !ok {
		return Identity{}, fmt.Errorf("unknown agent %q", agent)
	}
	return a.Identity(project, nativeID)
}
