package agentinit

// InitClaudeCode activates mnemo for Claude Code in the given project root.
// Agent instructions are installed globally by install.sh / install-instructions;
// project init only records activation in .mnemo.
func InitClaudeCode(root string) error {
	return AddAgent(root, "claudecode")
}
