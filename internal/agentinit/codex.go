package agentinit

// InitCodex activates mnemo for Codex in the given project root.
// Global hooks and instructions check for .mnemo at runtime.
func InitCodex(root string) error {
	return AddAgent(root, "codex")
}
