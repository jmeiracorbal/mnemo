// Package mnemo_cursor registers the Cursor agent adapter.
package mnemo_cursor

import (
	"errors"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
)

type cursorAdapter struct{ agents.BaseJSONAdapter }

func (a cursorAdapter) ReadNativeID(project string) (string, error) {
	data, err := os.ReadFile(agents.CursorSideChannelPath(project))
	if err != nil {
		return "", errors.New("cursor side-channel: " + err.Error())
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", errors.New("cursor side-channel: empty conversation_id")
	}
	return id, nil
}

func init() {
	agents.RegisterAdapter(agents.AgentCursor, cursorAdapter{
		BaseJSONAdapter: agents.BaseJSONAdapter{AgentType: agents.AgentCursor, Field: "conversation_id"},
	})
}
