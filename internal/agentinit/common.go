package agentinit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const markerName = ".mnemo"
const sectionStart = "<!-- mnemo:start -->"
const sectionEnd = "<!-- mnemo:end -->"

// Marker is the .mnemo file format.
type Marker struct {
	Version int      `json:"version"`
	Agents  []string `json:"agents"`
}

// ProjectRoot returns the git root of dir, or dir itself if not in a git repo.
func ProjectRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return dir
}

func readMarker(root string) (*Marker, error) {
	data, err := os.ReadFile(filepath.Join(root, markerName))
	if os.IsNotExist(err) {
		return &Marker{Version: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("malformed .mnemo file: %w", err)
	}
	return &m, nil
}

func writeMarker(root string, m *Marker) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, markerName), append(data, '\n'), 0644)
}

// AddAgent adds agent to the .mnemo marker, creating it if needed. Idempotent.
func AddAgent(root, agent string) error {
	m, err := readMarker(root)
	if err != nil {
		return err
	}
	for _, a := range m.Agents {
		if a == agent {
			return nil
		}
	}
	m.Agents = append(m.Agents, agent)
	return writeMarker(root, m)
}

// AppendSection appends a mnemo protocol section to path, creating the file if
// needed. Idempotent: does nothing if the section marker is already present.
func AppendSection(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(existing), sectionStart) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	section := "\n" + sectionStart + "\n" + content + "\n" + sectionEnd + "\n"
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		section = "\n" + section
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(section)
	return err
}

// WriteFile writes data to path, creating parent directories. Overwrites if exists.
func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// prependToFile prepends content before the existing content of path.
func prependToFile(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	combined := content
	if len(existing) > 0 {
		if !strings.HasSuffix(combined, "\n") {
			combined += "\n"
		}
		combined += "\n" + string(existing)
	}
	return os.WriteFile(path, []byte(combined), 0644)
}
