package cloudsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CloudConfigPath returns the canonical path for the cloud credentials file.
// It follows the XDG Base Directory Specification: ~/.config/mnemo/cloud.toml.
func CloudConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "mnemo", "cloud.toml"), nil
}

// LoadFileConfig reads the cloud credentials from ~/.config/mnemo/cloud.toml.
// Missing file returns an empty Config without error — the caller decides
// whether credentials are required.
func LoadFileConfig() (Config, error) {
	path, err := CloudConfigPath()
	if err != nil {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read cloud config %s: %w", path, err)
	}
	return parseCloudTOML(data)
}

// SaveFileConfig writes cfg to ~/.config/mnemo/cloud.toml, creating the
// directory if necessary.
func SaveFileConfig(cfg Config) error {
	path, err := CloudConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	content := formatCloudTOML(cfg)
	return os.WriteFile(path, []byte(content), 0600)
}

// DeleteFileConfig removes the cloud credentials file.
// Returns nil if the file did not exist.
func DeleteFileConfig() error {
	path, err := CloudConfigPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// FileConfigExists reports whether the cloud credentials file is present.
func FileConfigExists() bool {
	path, err := CloudConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// parseCloudTOML is a minimal parser for the [cloud] section of cloud.toml.
// It handles key = "value" pairs and ignores other sections and blank lines.
func parseCloudTOML(data []byte) (Config, error) {
	cfg := Config{}
	inCloud := false
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[cloud]" {
			inCloud = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inCloud = false
			continue
		}
		if !inCloud {
			continue
		}
		k, v, ok := parseTOMLKeyVal(line)
		if !ok {
			continue
		}
		switch k {
		case "provider":
			cfg.Provider = v
		case "url":
			cfg.URL = v
		case "key":
			cfg.Key = v
		case "client_id":
			cfg.ClientID = v
		}
	}
	return cfg, nil
}

// parseTOMLKeyVal parses a single "key = value" or 'key = "value"' line.
// Returns the key, unquoted value, and true on success.
func parseTOMLKeyVal(line string) (string, string, bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:idx])
	v := strings.TrimSpace(line[idx+1:])
	// Strip surrounding quotes.
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	} else if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		v = v[1 : len(v)-1]
	}
	return k, v, k != ""
}

func formatCloudTOML(cfg Config) string {
	provider := cfg.Provider
	if provider == "" {
		provider = "turso"
	}
	var b strings.Builder
	b.WriteString("[cloud]\n")
	b.WriteString("provider   = " + quoteVal(provider) + "\n")
	b.WriteString("url        = " + quoteVal(cfg.URL) + "\n")
	b.WriteString("key        = " + quoteVal(cfg.Key) + "\n")
	b.WriteString("client_id  = " + quoteVal(cfg.ClientID) + "\n")
	return b.String()
}

func quoteVal(s string) string { return `"` + s + `"` }
