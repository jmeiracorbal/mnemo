// Package plugin defines the Plugin interface and shared helpers for
// installing mnemo into different AI coding environments.
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Plugin installs and uninstalls mnemo in a specific coding environment.
type Plugin interface {
	Install(dryRun bool) error
	Uninstall() error
}

// ─── JSON helpers ─────────────────────────────────────────────────────────────

func ReadJSON(path string) (map[string]json.RawMessage, error) {
	config := make(map[string]json.RawMessage)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return config, nil
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return config, nil
}

func WriteJSON(path string, config map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// ─── Binary resolution ────────────────────────────────────────────────────────

// ResolveBinaryPath returns the absolute path of a binary, falling back to the name itself.
func ResolveBinaryPath(name string) string {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "bin", name),
		"/usr/local/bin/" + name,
		"/opt/homebrew/bin/" + name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}
