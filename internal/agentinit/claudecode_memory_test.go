package agentinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func defaultClaudeMemoryDir(home, projectRoot string) string {
	name := claudeProjectDirUnsafe.ReplaceAllString(projectRoot, "-")
	return filepath.Join(home, ".claude", "projects", name, "memory")
}

func TestClaudeAutoMemoryDirOutsideGitUsesRootPreservingCase(t *testing.T) {
	home := realTempDir(t)
	root := filepath.Join(realTempDir(t), "My.Project")
	mkdirAll(t, root)

	got := claudeAutoMemoryDir(home, root)
	if want := defaultClaudeMemoryDir(home, root); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
	if !strings.Contains(got, "My-Project") {
		t.Fatalf("dir %q must preserve case and replace dots", got)
	}
}

func TestClaudeAutoMemoryDirUsesConfigDir(t *testing.T) {
	home := realTempDir(t)
	configDir := realTempDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	root := realTempDir(t)

	name := claudeProjectDirUnsafe.ReplaceAllString(root, "-")
	if got, want := claudeAutoMemoryDir(home, root), filepath.Join(configDir, "projects", name, "memory"); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestClaudeAutoMemoryDirUsesGitRootFromSubdirectory(t *testing.T) {
	home := realTempDir(t)
	repo := realTempDir(t)
	mkdirAll(t, filepath.Join(repo, ".git"))
	root := filepath.Join(repo, "services", "api")
	mkdirAll(t, root)

	if got, want := claudeAutoMemoryDir(home, root), defaultClaudeMemoryDir(home, repo); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestClaudeAutoMemoryDirSharesMainCheckoutForWorktree(t *testing.T) {
	home := realTempDir(t)
	base := realTempDir(t)
	main := filepath.Join(base, "main")
	worktreeGitDir := filepath.Join(main, ".git", "worktrees", "feature")
	writeTestFile(t, filepath.Join(worktreeGitDir, "commondir"), "../..\n")
	worktree := filepath.Join(base, "feature")
	writeTestFile(t, filepath.Join(worktree, ".git"), "gitdir: "+worktreeGitDir+"\n")

	if got, want := claudeAutoMemoryDir(home, worktree), defaultClaudeMemoryDir(home, main); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestClaudeAutoMemoryDirKeepsSubmoduleRoot(t *testing.T) {
	home := realTempDir(t)
	parent := realTempDir(t)
	moduleGitDir := filepath.Join(parent, ".git", "modules", "lib")
	mkdirAll(t, moduleGitDir)
	submodule := filepath.Join(parent, "lib")
	writeTestFile(t, filepath.Join(submodule, ".git"), "gitdir: ../.git/modules/lib\n")

	if got, want := claudeAutoMemoryDir(home, submodule), defaultClaudeMemoryDir(home, submodule); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestClaudeAutoMemoryDirProjectDirNameRequiresConfigDir(t *testing.T) {
	home := realTempDir(t)
	root := realTempDir(t)
	t.Setenv("CLAUDE_CODE_PROJECT_DIR_NAME", "work")

	if got, want := claudeAutoMemoryDir(home, root), defaultClaudeMemoryDir(home, root); got != want {
		t.Fatalf("without CLAUDE_CONFIG_DIR: dir = %q, want %q", got, want)
	}

	configDir := realTempDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if got, want := claudeAutoMemoryDir(home, root), filepath.Join(configDir, "projects", "work", "memory"); got != want {
		t.Fatalf("with CLAUDE_CONFIG_DIR: dir = %q, want %q", got, want)
	}

	for _, invalid := range []string{"con", "has space", strings.Repeat("a", 65)} {
		t.Setenv("CLAUDE_CODE_PROJECT_DIR_NAME", invalid)
		if got := claudeAutoMemoryDir(home, root); strings.Contains(got, string(filepath.Separator)+invalid+string(filepath.Separator)) {
			t.Fatalf("invalid name %q used: %q", invalid, got)
		}
	}
}

func TestClaudeAutoMemoryDirMatchesHashedLongPath(t *testing.T) {
	home := realTempDir(t)
	root := filepath.Join(realTempDir(t), strings.Repeat("deep-directory-name/", 12))
	mkdirAll(t, root)
	name := claudeProjectDirUnsafe.ReplaceAllString(root, "-")
	if len(name) <= claudeProjectDirMaxLen {
		t.Fatalf("test path too short: %d", len(name))
	}
	projects := filepath.Join(home, ".claude", "projects")

	if got := claudeAutoMemoryDir(home, root); got != "" {
		t.Fatalf("dir without existing hashed directory = %q, want empty", got)
	}
	hashed := name[:claudeProjectDirMaxLen] + "-1a2b3c"
	mkdirAll(t, filepath.Join(projects, hashed))
	if got, want := claudeAutoMemoryDir(home, root), filepath.Join(projects, hashed, "memory"); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestClaudeAutoMemoryDirectorySettingPrecedence(t *testing.T) {
	home := realTempDir(t)
	repo := realTempDir(t)
	mkdirAll(t, filepath.Join(repo, ".git"))
	managed := realTempDir(t)
	old := claudeManagedSettingsDir
	claudeManagedSettingsDir = managed
	t.Cleanup(func() { claudeManagedSettingsDir = old })

	setting := func(dir string) string { return `{"autoMemoryDirectory":"` + dir + `"}` }
	steps := []struct {
		path string
		dir  string
		want string
	}{
		{filepath.Join(home, ".claude", "settings.json"), "~/user-memory", filepath.Join(home, "user-memory")},
		{filepath.Join(repo, ".claude", "settings.json"), "/project-memory", "/project-memory"},
		{filepath.Join(repo, ".claude", "settings.local.json"), "/local-memory", "/local-memory"},
		{filepath.Join(managed, "managed-settings.json"), "/managed-memory", "/managed-memory"},
		{filepath.Join(managed, "managed-settings.d", "10-memory.json"), "/dropin-10", "/dropin-10"},
		{filepath.Join(managed, "managed-settings.d", "20-memory.json"), "/dropin-20", "/dropin-20"},
	}
	for _, step := range steps {
		writeTestFile(t, step.path, setting(step.dir))
		if got := claudeAutoMemoryDir(home, repo); got != filepath.Clean(step.want) {
			t.Fatalf("after %s: dir = %q, want %q", step.path, got, step.want)
		}
	}
}

func TestClaudeAutoMemoryDirectoryIgnoresRelativeValue(t *testing.T) {
	home := realTempDir(t)
	root := realTempDir(t)
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), `{"autoMemoryDirectory":"relative/memory"}`)

	if got, want := claudeAutoMemoryDir(home, root), defaultClaudeMemoryDir(home, root); got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
}

func TestCheckCompetingMemoryDetectsClaudeAutoMemory(t *testing.T) {
	home := realTempDir(t)
	configDir := realTempDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	repo := realTempDir(t)
	mkdirAll(t, filepath.Join(repo, ".git"))
	root := filepath.Join(repo, "Sub.Dir")
	mkdirAll(t, root)
	if err := EnsureMarkerWithID(root, "project-123"); err != nil {
		t.Fatalf("marker: %v", err)
	}
	name := claudeProjectDirUnsafe.ReplaceAllString(repo, "-")
	memoryPath := filepath.Join(configDir, "projects", name, "memory", "MEMORY.md")
	writeTestFile(t, memoryPath, "# memory\n")

	check := CheckCompetingMemory(root, home)
	if check.Status != "warning" || !strings.Contains(check.Message, memoryPath) {
		t.Fatalf("check = %+v, want warning mentioning %s", check, memoryPath)
	}
}

func TestCheckCompetingMemoryDetectsCustomAutoMemoryDirectory(t *testing.T) {
	home := realTempDir(t)
	root := realTempDir(t)
	if err := EnsureMarkerWithID(root, "project-123"); err != nil {
		t.Fatalf("marker: %v", err)
	}
	custom := filepath.Join(realTempDir(t), "memory")
	writeTestFile(t, filepath.Join(root, ".claude", "settings.local.json"), `{"autoMemoryDirectory":"`+custom+`"}`)
	writeTestFile(t, filepath.Join(custom, "MEMORY.md"), "# memory\n")

	check := CheckCompetingMemory(root, home)
	if check.Status != "warning" || !strings.Contains(check.Message, filepath.Join(custom, "MEMORY.md")) {
		t.Fatalf("check = %+v, want warning mentioning custom MEMORY.md", check)
	}
}
