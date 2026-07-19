package agentinit

// InitOpenCode activates mnemo for OpenCode in the given project root.
// The OpenCode plugin is installed globally and checks for .mnemo at runtime.
func InitOpenCode(root string) error {
	return AddAgent(root, "opencode")
}
