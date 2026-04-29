package agentinit

import (
	_ "embed"
	"encoding/json"
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
	if _, err := os.Stat(stop); os.IsNotExist(err) {
		return fmt.Errorf("cursor init: hook script not found: %s\nRun install.sh --agent=cursor first", stop)
	}

	hooksData := map[string]any{
		"version": 1,
		"hooks": map[string]any{
			"beforeSubmitPrompt": []map[string]string{{"command": beforeSubmit}},
			"stop":               []map[string]string{{"command": stop}},
		},
	}
	hooksJSON, err := json.MarshalIndent(hooksData, "", "  ")
	if err != nil {
		return fmt.Errorf("cursor init: marshal hooks.json: %w", err)
	}

	hooksPath := filepath.Join(root, ".cursor", "hooks.json")
	if err := WriteFile(hooksPath, append(hooksJSON, '\n')); err != nil {
		return fmt.Errorf("cursor init: .cursor/hooks.json: %w", err)
	}

	rulesPath := filepath.Join(root, ".cursor", "rules", "mnemo.mdc")
	if err := WriteFile(rulesPath, cursorProtocol); err != nil {
		return fmt.Errorf("cursor init: .cursor/rules/mnemo.mdc: %w", err)
	}

	return AddAgent(root, "cursor")
}
