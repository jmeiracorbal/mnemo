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
const globalSkillName = "mnemo-memory"

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

// EnsureGitignore adds .mnemo to the project .gitignore if not already present.
// Creates .gitignore if it does not exist. Idempotent.
func EnsureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".mnemo" {
			return nil
		}
	}
	entry := ".mnemo\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		entry = "\n" + entry
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
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

// GlobalSkillPath returns the canonical path used by the skills CLI for a
// globally installed mnemo-memory skill.
func GlobalSkillPath(home string) string {
	return filepath.Join(home, ".agents", "skills", globalSkillName, "SKILL.md")
}

// GlobalSkillInstalled reports whether the canonical global skill file exists.
func GlobalSkillInstalled(home string) bool {
	info, err := os.Stat(GlobalSkillPath(home))
	return err == nil && !info.IsDir()
}

// AppendSection creates or updates the managed mnemo protocol section in path.
// Content outside the markers is preserved.
func AppendSection(path, content string) error {
	return upsertManagedSection(path, sectionStart, sectionEnd, content, "")
}

// AppendClaudeSection creates or updates the Claude-specific managed section.
// It also adds @AGENTS.md once when creating the section.
func AppendClaudeSection(path, content string) error {
	return upsertManagedSection(path, claudeSectionStart, claudeSectionEnd, content, "@AGENTS.md")
}

func upsertManagedSection(path, startMarker, endMarker, content, prelude string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	current := string(existing)
	startCount := strings.Count(current, startMarker)
	endCount := strings.Count(current, endMarker)
	if startCount != endCount || startCount > 1 {
		return fmt.Errorf(
			"malformed managed section: found %d %q marker(s) and %d %q marker(s)",
			startCount, startMarker, endCount, endMarker,
		)
	}

	block := startMarker + "\n" + strings.TrimRight(content, "\n") + "\n" + endMarker
	var updated string

	if startCount == 1 {
		start := strings.Index(current, startMarker)
		end := strings.Index(current, endMarker)
		if end < start {
			return fmt.Errorf("malformed managed section: %q appears before %q", endMarker, startMarker)
		}
		end += len(endMarker)
		updated = current[:start] + block + current[end:]
	} else {
		addition := block
		if prelude != "" && !containsLine(current, prelude) {
			addition = prelude + "\n\n" + addition
		}
		updated = appendSection(current, addition)
	}

	if updated == current {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0644)
}

func containsLine(content, target string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

func appendSection(existing, addition string) string {
	if existing == "" {
		return addition + "\n"
	}
	if strings.HasSuffix(existing, "\n\n") {
		return existing + addition + "\n"
	}
	if strings.HasSuffix(existing, "\n") {
		return existing + "\n" + addition + "\n"
	}
	return existing + "\n\n" + addition + "\n"
}

// WriteFile writes data to path, creating parent directories. Overwrites if exists.
func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
