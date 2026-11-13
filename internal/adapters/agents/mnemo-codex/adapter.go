// Package mnemo_codex registers the Codex agent adapter.
package mnemo_codex

import (
	"errors"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
)

type codexAdapter struct{ agents.BaseJSONAdapter }

func (a codexAdapter) ParseToolCallMeta(meta map[string]any, project string) (agents.Identity, error) {
	raw, ok := meta["x-codex-turn-metadata"]
	if !ok {
		return agents.Identity{}, errors.New("codex tools/call metadata requires x-codex-turn-metadata")
	}
	turn, ok := raw.(map[string]any)
	if !ok {
		return agents.Identity{}, errors.New("codex tools/call x-codex-turn-metadata must be an object")
	}
	nativeID, ok := turn["session_id"].(string)
	if !ok || strings.TrimSpace(nativeID) == "" {
		return agents.Identity{}, errors.New("codex tools/call metadata requires session_id")
	}
	return a.Identity(project, nativeID)
}

func init() {
	agents.RegisterAdapter(agents.AgentCodex, codexAdapter{
		BaseJSONAdapter: agents.BaseJSONAdapter{AgentType: agents.AgentCodex, Field: "session_id"},
	})
}
