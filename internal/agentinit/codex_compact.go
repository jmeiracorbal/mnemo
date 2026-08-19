package agentinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mnemoassets "github.com/jmeiracorbal/mnemo"
)

func writeCodexCompactionPrompt(home string) ([]string, error) {
	data, err := mnemoassets.SetupAssets.ReadFile("assets/protocol/codex-compact-prompt.md")
	if err != nil {
		return nil, fmt.Errorf("read codex compact prompt asset: %w", err)
	}

	promptPath := filepath.Join(home, ".codex", "mnemo-compact-prompt.md")
	if err := WriteFile(promptPath, data); err != nil {
		return nil, fmt.Errorf("write codex compact prompt: %w", err)
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := upsertCodexCompactPromptFile(configPath, promptPath); err != nil {
		return nil, err
	}

	return []string{promptPath, configPath}, nil
}

func upsertCodexCompactPromptFile(path, promptPath string) error {
	line := fmt.Sprintf("experimental_compact_prompt_file = %q", promptPath)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	var out []string
	replaced := false
	for _, lineText := range lines {
		trimmed := strings.TrimSpace(lineText)
		if strings.HasPrefix(trimmed, "experimental_compact_prompt_file") {
			if !replaced {
				out = append(out, line)
				replaced = true
			}
			continue
		}
		out = append(out, lineText)
	}

	content := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if !replaced {
		if content != "" {
			content += "\n\n"
		}
		content += line
	}
	content += "\n"

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
