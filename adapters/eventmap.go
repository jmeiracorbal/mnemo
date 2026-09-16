package adapters

import (
	"encoding/json"
	"fmt"
)

// LifecycleAction constants name the plugin-level actions that certain events
// trigger. Plugins define what each action means in their own context; Go
// declares which events should trigger them.
const (
	LifecycleActionStartSession = "start_session"
)

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

// AgentEventMap is the per-agent event translation contract. When a platform
// renames an event, update this map and plugins adapt on next load.
type AgentEventMap struct {
	Agent  Agent          `json:"agent"`
	Events []EventMapping `json:"events"`
}

var agentEventMaps = map[Agent]AgentEventMap{
	AgentOpenCode: {
		Agent: AgentOpenCode,
		Events: []EventMapping{
			{NativeType: "session.created", MnemoType: "execution.started", NativeIDPath: "info.id", PayloadContext: "directory", LifecycleAction: LifecycleActionStartSession},
			{NativeType: "session.compacted", MnemoType: "session.compacted", NativeIDPath: "info.id"},
		},
	},
	AgentPi: {
		Agent: AgentPi,
		Events: []EventMapping{
			{NativeType: "session_start", MnemoType: "execution.started", PayloadContext: "directory", LifecycleAction: LifecycleActionStartSession},
			{NativeType: "session_shutdown", MnemoType: "execution.closed"},
			{NativeType: "session_compact", MnemoType: "session.compacted"},
		},
	},
}

// EventMapFor returns the event mapping for one agent.
func EventMapFor(agent Agent) (AgentEventMap, bool) {
	m, ok := agentEventMaps[agent]
	return m, ok
}

// AllEventMaps returns every registered agent event map.
func AllEventMaps() []AgentEventMap {
	result := make([]AgentEventMap, 0, len(agentEventMaps))
	for _, m := range agentEventMaps {
		result = append(result, m)
	}
	return result
}

// EventMapJSON serialises one agent's event map, or all maps when agent is empty.
func EventMapJSON(agent Agent) ([]byte, error) {
	if agent != "" {
		m, ok := agentEventMaps[agent]
		if !ok {
			return nil, fmt.Errorf("no event map for agent %q", agent)
		}
		return json.MarshalIndent(m, "", "  ")
	}
	return json.MarshalIndent(AllEventMaps(), "", "  ")
}
