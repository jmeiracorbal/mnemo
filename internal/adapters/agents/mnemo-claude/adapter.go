// Package mnemo_claude registers the Claude Code agent adapter.
package mnemo_claude

import "github.com/jmeiracorbal/mnemo/internal/adapters/agents"

type claudeAdapter struct{ agents.BaseJSONAdapter }

func (claudeAdapter) NativeIDEnvironmentVariable() string { return "CLAUDE_CODE_SESSION_ID" }

func init() {
	agents.RegisterAdapter(agents.AgentClaudeCode, claudeAdapter{
		BaseJSONAdapter: agents.BaseJSONAdapter{AgentType: agents.AgentClaudeCode, Field: "session_id"},
	})
}
