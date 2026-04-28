package agentinit

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/cursor.mdc
var cursorProtocol []byte

// InitCursor configures mnemo for Cursor in the given project root.
//
// Writes .cursor/hooks.json referencing the global hook scripts and
// .cursor/rules/mnemo.mdc with the mnemo protocol.
func InitCursor(root string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cursor init: could not determine HOME: %w", err)
	}

	hooksDir := filepath.Join(home, ".cursor", "hooks")
	beforeSubmit := filepath.Join(hooksDir, "before-submit-prompt.sh")
	stop := filepath.Join(hooksDir, "stop.sh")

	if _, err := os.Stat(beforeSubmit); os.IsNotExist(err) {
		return fmt.Errorf("cursor init: hook script not found: %s\nRun install.sh --agent=cursor first", beforeSubmit)
	}

	hooksJSON := fmt.Sprintf(`{
  "version": 1,
  "hooks": {
    "beforeSubmitPrompt": [{"command": "%s"}],
    "stop": [{"command": "%s"}]
  }
}
`, beforeSubmit, stop)

	hooksPath := filepath.Join(root, ".cursor", "hooks.json")
	if err := WriteFile(hooksPath, []byte(hooksJSON)); err != nil {
		return fmt.Errorf("cursor init: .cursor/hooks.json: %w", err)
	}

	rulesPath := filepath.Join(root, ".cursor", "rules", "mnemo.mdc")
	if err := WriteFile(rulesPath, cursorProtocol); err != nil {
		return fmt.Errorf("cursor init: .cursor/rules/mnemo.mdc: %w", err)
	}

	return AddAgent(root, "cursor")
}
