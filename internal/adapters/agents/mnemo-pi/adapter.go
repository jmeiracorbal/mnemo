// Package mnemo_pi registers the Pi agent adapter and its event mappings.
package mnemo_pi

import (
	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	"github.com/jmeiracorbal/mnemo/internal/adapters/mapping"
)

func init() {
	agents.RegisterAdapter(agents.AgentPi, agents.BaseDirectAdapter{
		AgentType: agents.AgentPi,
	})
	mapping.Register(mapping.AgentEventMap{
		Agent: agents.AgentPi,
		Events: []mapping.EventMapping{
			{NativeType: "session_start", MnemoType: "execution.started", PayloadContext: "directory", LifecycleAction: mapping.LifecycleActionStartSession},
			{NativeType: "session_shutdown", MnemoType: "execution.closed"},
			{NativeType: "session_compact", MnemoType: "session.compacted"},
		},
	})
}
