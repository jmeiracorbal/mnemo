package agentinit

// InitCursor activates mnemo for Cursor in the given project root.
// Global hooks and instructions check for .mnemo at runtime.
func InitCursor(root string) error {
	return AddAgent(root, "cursor")
}
