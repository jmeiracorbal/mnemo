package agentinit

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/windsurf.md
var windsurfProtocol []byte

// InitWindsurf configures mnemo for Windsurf in the given project root.
//
// Writes .windsurf/hooks.json referencing the global hook scripts and
// .windsurf/rules/mnemo.md with the mnemo protocol.
func InitWindsurf(root string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("windsurf init: could not determine HOME: %w", err)
	}

	hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
	prePrompt := filepath.Join(hooksDir, "pre-user-prompt.sh")
	postCascade := filepath.Join(hooksDir, "post-cascade-response.sh")

	if _, err := os.Stat(prePrompt); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("windsurf init: hook script not found: %s\nRun install.sh --agent=windsurf first", prePrompt)
		}
		return fmt.Errorf("windsurf init: stat %s: %w", prePrompt, err)
	}
	if _, err := os.Stat(postCascade); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("windsurf init: hook script not found: %s\nRun install.sh --agent=windsurf first", postCascade)
		}
		return fmt.Errorf("windsurf init: stat %s: %w", postCascade, err)
	}

	hooksData := map[string]any{
		"hooks": map[string]any{
			"pre_user_prompt":                       []map[string]string{{"command": prePrompt}},
			"post_cascade_response_with_transcript": []map[string]string{{"command": postCascade}},
		},
	}
	hooksJSON, err := json.MarshalIndent(hooksData, "", "  ")
	if err != nil {
		return fmt.Errorf("windsurf init: marshal hooks.json: %w", err)
	}

	hooksPath := filepath.Join(root, ".windsurf", "hooks.json")
	if err := WriteFile(hooksPath, append(hooksJSON, '\n')); err != nil {
		return fmt.Errorf("windsurf init: .windsurf/hooks.json: %w", err)
	}

	rulesPath := filepath.Join(root, ".windsurf", "rules", "mnemo.md")
	if err := WriteFile(rulesPath, windsurfProtocol); err != nil {
		return fmt.Errorf("windsurf init: .windsurf/rules/mnemo.md: %w", err)
	}

	return AddAgent(root, "windsurf")
}
