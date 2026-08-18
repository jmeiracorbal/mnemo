package agentinit

import (
	"os"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/templates"
)

func cursorLabel() string { return "Cursor" }

func cursorDetectionPaths(home string) []string {
	return []string{filepath.Join(home, ".cursor")}
}

func cursorInstructionPath(home string) string {
	return filepath.Join(home, ".cursor", "rules", "mnemo.mdc")
}

func cursorInstallInstructions(home string) (string, error) {
	path := cursorInstructionPath(home)
	if err := WriteFile(path, []byte(templates.CursorGlobal)); err != nil {
		return "", err
	}
	return path, nil
}

func cursorRemoveInstructions(home string) (string, bool, error) {
	path := cursorInstructionPath(home)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return path, false, nil
		}
		return path, false, err
	}
	return path, true, nil
}

func cursorConfigSnippets(home, mnemoBin string) []ConfigSnippet {
	hooksDir := filepath.Join(home, ".cursor", "hooks")
	return []ConfigSnippet{
		{
			Agent:   cursorLabel(),
			Path:    filepath.Join(home, ".cursor", "mcp.json"),
			Format:  "json",
			Content: mcpServersJSON(mnemoBin),
		},
		{
			Agent:  cursorLabel(),
			Path:   filepath.Join(home, ".cursor", "hooks.json"),
			Format: "json",
			Content: prettyJSON(map[string]any{
				"version": 1,
				"hooks": map[string]any{
					"beforeSubmitPrompt": []any{map[string]any{"command": filepath.Join(hooksDir, "before-submit-prompt.sh")}},
					"stop":               []any{map[string]any{"command": filepath.Join(hooksDir, "stop.sh")}},
				},
			}),
		},
	}
}

func cursorRuntimeAssets() []assetTarget {
	return []assetTarget{
		{Asset: "scripts/cursor/hooks/before-submit-prompt.sh", Path: filepath.Join(".cursor", "hooks", "before-submit-prompt.sh"), Mode: 0755},
		{Asset: "scripts/cursor/hooks/stop.sh", Path: filepath.Join(".cursor", "hooks", "stop.sh"), Mode: 0755},
	}
}

func cursorUninstallConfig(home string) ([]string, error) {
	var removed []string
	mcpPath := filepath.Join(home, ".cursor", "mcp.json")
	changed, err := removeMCPServer(mcpPath, "mcpServers", "mnemo")
	paths, err := appendChanged(mcpPath, changed, err)
	removed = append(removed, paths...)
	if err != nil {
		return removed, err
	}

	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	hooksDir := filepath.Join(home, ".cursor", "hooks")
	changed, err = removeHookCommands(hooksPath, map[string]string{
		"beforeSubmitPrompt": filepath.Join(hooksDir, "before-submit-prompt.sh"),
		"stop":               filepath.Join(hooksDir, "stop.sh"),
	})
	paths, err = appendChanged(hooksPath, changed, err)
	removed = append(removed, paths...)
	return removed, err
}

func cursorCheckInstructions(home string) Check {
	return checkInstructionFile("cursor", cursorInstructionPath(home), false)
}

func cursorCheckMCP(home string) Check {
	path := filepath.Join(home, ".cursor", "mcp.json")
	return checkJSONHas(path, "mcp_config.cursor", "cursor", "mcpServers", "mnemo")
}

func cursorCheckRuntime(home string) Check {
	paths := []string{
		filepath.Join(home, ".cursor", "hooks.json"),
		filepath.Join(home, ".cursor", "hooks", "before-submit-prompt.sh"),
		filepath.Join(home, ".cursor", "hooks", "stop.sh"),
	}
	return checkFiles("cursor", "runtime_files.cursor", "Cursor global hooks installed", paths, true)
}
