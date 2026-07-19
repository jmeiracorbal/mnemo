package agentinit

// InitWindsurf activates mnemo for Windsurf in the given project root.
// Global hooks and instructions check for .mnemo at runtime.
func InitWindsurf(root string) error {
	return AddAgent(root, "windsurf")
}
