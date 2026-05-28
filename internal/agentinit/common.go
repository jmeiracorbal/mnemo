package agentinit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const markerName = ".mnemo"

// mnemoNamespace is the UUID v5 namespace for mnemo project identifiers.
// Fixed forever — changing this would invalidate all existing project IDs.
var mnemoNamespace = uuid.MustParse("6f3e4b2a-1c5d-4e7f-8a9b-0c1d2e3f4a5b")

// ProjectUUIDFromPath returns the deterministic UUID v5 for an absolute project path.
// Same path always produces the same UUID. Used by both init and migrate.
// Panics if absPath is not absolute to prevent non-deterministic IDs.
func ProjectUUIDFromPath(absPath string) string {
	if !filepath.IsAbs(absPath) {
		panic("ProjectUUIDFromPath: path must be absolute, got: " + absPath)
	}
	return uuid.NewSHA1(mnemoNamespace, []byte(absPath)).String()
}
const sectionStart = "<!-- mnemo:start -->"
const sectionEnd = "<!-- mnemo:end -->"
const claudeSectionStart = "<!-- mnemo:claude-start -->"
const claudeSectionEnd = "<!-- mnemo:claude-end -->"

// Marker is the .mnemo file format.
type Marker struct {
	Version int      `json:"version"`
	ID      string   `json:"id,omitempty"`
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

// EnsureProjectID returns the project UUID from .mnemo. If not set, derives it
// deterministically from the absolute path, persists it, and returns it.
func EnsureProjectID(root string) (string, error) {
	m, err := readMarker(root)
	if err != nil {
		return "", err
	}
	if m.ID != "" {
		return m.ID, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	m.ID = ProjectUUIDFromPath(abs)
	if err := writeMarker(root, m); err != nil {
		return "", fmt.Errorf("write project ID to .mnemo: %w", err)
	}
	return m.ID, nil
}

// ReadProjectID returns the project UUID from .mnemo.
// Errors if .mnemo does not exist or has no ID set.
func ReadProjectID(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, markerName))
	if os.IsNotExist(err) {
		return "", fmt.Errorf(".mnemo not found in %s — run mnemo init first", root)
	}
	if err != nil {
		return "", err
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("malformed .mnemo: %w", err)
	}
	if m.ID == "" {
		return "", fmt.Errorf(".mnemo has no project ID in %s — run mnemo init", root)
	}
	return m.ID, nil
}

// EnsureMarkerWithID writes id into the .mnemo at root, preserving existing
// fields. Creates the file if absent. Used by migrate-projects.
func EnsureMarkerWithID(root, id string) error {
	m, err := readMarker(root)
	if err != nil {
		return err
	}
	m.ID = id
	return writeMarker(root, m)
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
	_, writeErr := f.WriteString(section)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// AppendClaudeSection appends an @AGENTS.md include and the Claude-specific mnemo
// section to path, creating the file if needed. Idempotent: does nothing if the
// Claude section marker is already present.
func AppendClaudeSection(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(existing), claudeSectionStart) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	section := "\n@AGENTS.md\n\n" + claudeSectionStart + "\n" + content + "\n" + claudeSectionEnd + "\n"
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		section = "\n" + section
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString(section)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// WriteFile writes data to path, creating parent directories. Overwrites if exists.
func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
