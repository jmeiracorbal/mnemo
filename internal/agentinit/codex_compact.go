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
	const key = "experimental_compact_prompt_file"
	line := fmt.Sprintf("experimental_compact_prompt_file = %q", promptPath)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+1)
	for _, lineText := range lines {
		trimmed := strings.TrimSpace(lineText)
		if gotKey, _, ok := tomlAssignment(trimmed); ok && gotKey == key {
			continue
		}
		out = append(out, lineText)
	}

	insertAt := len(out)
	for i, lineText := range out {
		if isTOMLTableHeader(strings.TrimSpace(lineText)) {
			insertAt = i
			break
		}
	}
	out = append(out, "")
	copy(out[insertAt+1:], out[insertAt:])
	out[insertAt] = line
	content := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
