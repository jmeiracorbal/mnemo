// Package mapping defines the per-agent event translation contract and a
// thread-safe registry that external integrators extend via Register.
package mapping

import "github.com/jmeiracorbal/mnemo/internal/adapters/agents"

const LifecycleActionStartSession = "start_session"

// EventMapping maps one native agent lifecycle event to a mnemo durable event.
// NativeIDPath is a dot-separated path into the event properties object; an
// empty value means the native ID is provided by the agent's context API.
// PayloadContext is an optional hint for context-dependent payload fields:
// "directory" means include the workspace directory in the event payload.
// LifecycleAction names a plugin-level behaviour beyond a plain publish; empty
// means the event only needs to be published.
type EventMapping struct {
	NativeType      string `json:"native_type"`
	MnemoType       string `json:"mnemo_type"`
	NativeIDPath    string `json:"native_id_path,omitempty"`
	PayloadContext  string `json:"payload_context,omitempty"`
	LifecycleAction string `json:"lifecycle_action,omitempty"`
}

// AgentEventMap is the per-agent event translation contract.
type AgentEventMap struct {
	Agent  agents.Agent   `json:"agent"`
	Events []EventMapping `json:"events"`
}
