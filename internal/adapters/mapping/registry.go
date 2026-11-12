package mapping

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
)

var (
	mu       sync.RWMutex
	registry = map[agents.Agent]AgentEventMap{}
)

// Register adds or replaces the event map for one agent. External integrators
// call this from their own init() to register their agent event mappings.
func Register(m AgentEventMap) {
	mu.Lock()
	defer mu.Unlock()
	registry[m.Agent] = m
}

// EventMapFor returns the event mapping for one agent.
func EventMapFor(agent agents.Agent) (AgentEventMap, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := registry[agent]
	return m, ok
}

// AllEventMaps returns every registered agent event map.
func AllEventMaps() []AgentEventMap {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]AgentEventMap, 0, len(registry))
	for _, m := range registry {
		result = append(result, m)
	}
	return result
}

// EventMapJSON serialises one agent's event map, or all maps when agent is empty.
func EventMapJSON(agent agents.Agent) ([]byte, error) {
	if agent != "" {
		m, ok := EventMapFor(agent)
		if !ok {
			return nil, fmt.Errorf("no event map for agent %q", agent)
		}
		return json.MarshalIndent(m, "", "  ")
	}
	return json.MarshalIndent(AllEventMaps(), "", "  ")
}
