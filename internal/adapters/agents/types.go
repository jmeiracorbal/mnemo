// Package agents defines the agent identity contract for mnemo integrations.
package agents

// Agent identifies a supported agent platform.
type Agent string

const (
	AgentClaudeCode Agent = "claudecode"
	AgentCodex      Agent = "codex"
	AgentCursor     Agent = "cursor"
	AgentOpenCode   Agent = "opencode"
	AgentPi         Agent = "pi"
)
