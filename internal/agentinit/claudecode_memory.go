package agentinit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// Claude Code auto memory resolution, as documented in
// https://code.claude.com/docs/en/memory#storage-location and
// https://code.claude.com/docs/en/sessions#where-transcripts-are-stored.
// Managed settings delivered through MDM, the registry or the claude.ai
// console, and values passed with --settings, are not visible on disk and
// cannot be inspected here.

const claudeProjectDirMaxLen = 200

var (
	claudeProjectDirUnsafe  = regexp.MustCompile(`[^a-zA-Z0-9]`)
	claudeProjectDirNameEnv = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	windowsDeviceName       = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)

	// claudeManagedSettingsDir is the system directory holding
	// managed-settings.json; empty disables managed settings lookup.
	claudeManagedSettingsDir = defaultClaudeManagedSettingsDir()
)

func defaultClaudeManagedSettingsDir() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "windows":
		return `C:\Program Files\ClaudeCode`
	default:
		return "/etc/claude-code"
	}
}

// claudeNativeMemoryPath returns the auto memory MEMORY.md Claude Code would
// load for the project at root, or "" when it does not exist.
func claudeNativeMemoryPath(home, root string) string {
	dir := claudeAutoMemoryDir(home, root)
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, "MEMORY.md")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// claudeAutoMemoryDir resolves the auto memory directory: an
// autoMemoryDirectory setting wins; otherwise it is
// <config dir>/projects/<project>/memory, where <project> derives from the
// canonical git root shared by every worktree and subdirectory.
func claudeAutoMemoryDir(home, root string) string {
	topLevel, projectRoot := claudeProjectRoots(root)
	if dir := claudeAutoMemoryDirectorySetting(home, root, topLevel, projectRoot); dir != "" {
		return dir
	}
	projects := filepath.Join(claudeConfigDir(home), "projects")
	name := claudeProjectDirName(projects, projectRoot)
	if name == "" {
		return ""
	}
	return filepath.Join(projects, name, "memory")
}

// claudeProjectDirName mirrors Claude Code's <project> directory name.
func claudeProjectDirName(projectsDir, projectRoot string) string {
	if os.Getenv("CLAUDE_CONFIG_DIR") != "" {
		if name := os.Getenv("CLAUDE_CODE_PROJECT_DIR_NAME"); claudeProjectDirNameEnv.MatchString(name) && !windowsDeviceName.MatchString(name) {
			return name
		}
	}
	name := claudeProjectDirUnsafe.ReplaceAllString(projectRoot, "-")
	if len(name) <= claudeProjectDirMaxLen {
		return name
	}
	// Longer names are truncated and suffixed with a hash of the full path;
	// match the existing directory by its truncated prefix.
	prefix := name[:claudeProjectDirMaxLen] + "-"
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			return entry.Name()
		}
	}
	return ""
}

// claudeProjectRoots returns the git top-level of root and the canonical
// project root: the main checkout for a linked worktree, the top-level for a
// regular repository, and root itself outside git.
func claudeProjectRoots(root string) (topLevel, projectRoot string) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return root, root
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	for dir := abs; ; {
		info, err := os.Stat(filepath.Join(dir, ".git"))
		if err == nil {
			if info.IsDir() {
				return dir, dir
			}
			return dir, gitWorktreeMainRoot(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs, abs
		}
		dir = parent
	}
}

// gitWorktreeMainRoot follows a .git file to the main checkout. Submodules and
// unreadable links keep their own directory as the root.
func gitWorktreeMainRoot(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return dir
	}
	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return dir
	}
	gitDir = resolveFrom(dir, strings.TrimSpace(gitDir))
	common, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return dir
	}
	commonDir := resolveFrom(gitDir, strings.TrimSpace(string(common)))
	if filepath.Base(commonDir) != ".git" {
		return dir
	}
	return filepath.Dir(commonDir)
}

func resolveFrom(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

// claudeAutoMemoryDirectorySetting returns autoMemoryDirectory from the
// highest-precedence settings file that sets a valid value: managed, then
// project local, shared project and user settings.
func claudeAutoMemoryDirectorySetting(home, root, topLevel, projectRoot string) string {
	var files []string
	if claudeManagedSettingsDir != "" {
		// managed-settings.json is merged first and managed-settings.d/*.json
		// after it in alphabetical order, so the last drop-in wins.
		dropIns, _ := filepath.Glob(filepath.Join(claudeManagedSettingsDir, "managed-settings.d", "*.json"))
		sort.Sort(sort.Reverse(sort.StringSlice(dropIns)))
		for _, path := range dropIns {
			if !strings.HasPrefix(filepath.Base(path), ".") {
				files = append(files, path)
			}
		}
		files = append(files, filepath.Join(claudeManagedSettingsDir, "managed-settings.json"))
	}
	files = append(files, filepath.Join(projectRoot, ".claude", "settings.local.json"))
	for _, dir := range uniquePaths(root, topLevel, projectRoot) {
		files = append(files, filepath.Join(dir, ".claude", "settings.json"))
	}
	files = append(files, filepath.Join(claudeConfigDir(home), "settings.json"))

	for _, path := range files {
		if dir := readClaudeAutoMemoryDirectory(path, home); dir != "" {
			return dir
		}
	}
	return ""
}

func readClaudeAutoMemoryDirectory(path, home string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var settings struct {
		AutoMemoryDirectory string `json:"autoMemoryDirectory"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return ""
	}
	dir := settings.AutoMemoryDirectory
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		dir = filepath.Join(home, rest)
	}
	// Claude Code accepts only absolute or ~/-prefixed values.
	if !filepath.IsAbs(dir) {
		return ""
	}
	return filepath.Clean(dir)
}

func uniquePaths(paths ...string) []string {
	seen := make(map[string]struct{}, len(paths))
	var out []string
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}
