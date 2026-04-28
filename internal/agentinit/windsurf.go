package agentinit

import (
	_ "embed"
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

	if _, err := os.Stat(prePrompt); os.IsNotExist(err) {
		return fmt.Errorf("windsurf init: hook script not found: %s\nRun install.sh --agent=windsurf first", prePrompt)
	}

	hooksJSON := fmt.Sprintf(`{
  "hooks": {
    "pre_user_prompt": [{"command": "%s"}],
    "post_cascade_response_with_transcript": [{"command": "%s"}]
  }
}
`, prePrompt, postCascade)

	hooksPath := filepath.Join(root, ".windsurf", "hooks.json")
	if err := WriteFile(hooksPath, []byte(hooksJSON)); err != nil {
		return fmt.Errorf("windsurf init: .windsurf/hooks.json: %w", err)
	}

	rulesPath := filepath.Join(root, ".windsurf", "rules", "mnemo.md")
	if err := WriteFile(rulesPath, windsurfProtocol); err != nil {
		return fmt.Errorf("windsurf init: .windsurf/rules/mnemo.md: %w", err)
	}

	return AddAgent(root, "windsurf")
}
