// Package mnemo_opencode registers the OpenCode agent adapter and its event mappings.
package mnemo_opencode

import (
	"errors"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	"github.com/jmeiracorbal/mnemo/internal/adapters/mapping"
)

type openCodeAdapter struct{ agents.BaseDirectAdapter }

func (a openCodeAdapter) ReadNativeID(project string) (string, error) {
	data, err := os.ReadFile(agents.OpenCodeSideChannelPath(project))
	if err != nil {
		return "", errors.New("opencode side-channel: " + err.Error())
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", errors.New("opencode side-channel: empty session_id")
	}
	return id, nil
}

func init() {
	agents.RegisterAdapter(agents.AgentOpenCode, openCodeAdapter{
		BaseDirectAdapter: agents.BaseDirectAdapter{AgentType: agents.AgentOpenCode},
	})
	mapping.Register(mapping.AgentEventMap{
		Agent: agents.AgentOpenCode,
		Events: []mapping.EventMapping{
			{NativeType: "session.created", MnemoType: "execution.started", NativeIDPath: "info.id", PayloadContext: "directory", LifecycleAction: mapping.LifecycleActionStartSession},
			{NativeType: "session.compacted", MnemoType: "session.compacted", NativeIDPath: "info.id"},
		},
	})
}
