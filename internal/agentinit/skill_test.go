package agentinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCompetingMemoryDetectsProjectMEMORYmd(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	if err := EnsureMarkerWithID(root, "project-123"); err != nil {
		t.Fatalf("marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "MEMORY.md"), []byte("# memory\n"), 0644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	check := CheckCompetingMemory(root, home)
	if check.Status != "warning" {
		t.Fatalf("status = %q, want warning", check.Status)
	}
	if !strings.Contains(check.Message, "MEMORY.md") {
		t.Fatalf("message = %q, want MEMORY.md mention", check.Message)
	}
}

func TestInstallGlobalSkillCopiesCanonicalFiles(t *testing.T) {
	home := t.TempDir()
	paths, err := InstallGlobalSkill(home)
	if err != nil {
		t.Fatalf("install global skill: %v", err)
	}
	if len(paths) < 2 {
		t.Fatalf("expected skill paths, got %v", paths)
	}
	if !GlobalSkillInstalled(home) {
		t.Fatal("global skill not detected after install")
	}
	for _, link := range []string{
		filepath.Join(home, ".claude", "skills", globalSkillName),
		filepath.Join(home, ".codeium", "windsurf", "skills", globalSkillName),
		filepath.Join(home, ".pi", "agent", "skills", globalSkillName),
	} {
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("stat skill link %s: %v", link, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink", link)
		}
	}
}

func TestUpsertCodexCompactPromptFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	promptPath := filepath.Join(dir, "mnemo-compact-prompt.md")

	if err := upsertCodexCompactPromptFile(configPath, promptPath); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "experimental_compact_prompt_file") || !strings.Contains(string(data), promptPath) {
		t.Fatalf("config missing compact prompt setting: %s", data)
	}

	if err := os.WriteFile(configPath, []byte("model = \"gpt\"\nexperimental_compact_prompt_file = \"/old/path\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := upsertCodexCompactPromptFile(configPath, promptPath); err != nil {
		t.Fatalf("replace upsert: %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "experimental_compact_prompt_file") != 1 {
		t.Fatalf("expected one compact prompt key, got: %s", data)
	}

	if err := os.WriteFile(configPath, []byte("[mcp_servers.mnemo.env]\nMNEMO_AGENT = \"codex\"\nexperimental_compact_prompt_file = \"/old/scoped/path\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := upsertCodexCompactPromptFile(configPath, promptPath); err != nil {
		t.Fatalf("move scoped upsert: %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, "experimental_compact_prompt_file") != 1 {
		t.Fatalf("expected one compact prompt key after scoped move, got: %s", content)
	}
	if strings.Index(content, "experimental_compact_prompt_file") > strings.Index(content, "[mcp_servers.mnemo.env]") {
		t.Fatalf("compact prompt key must be top-level before TOML tables, got: %s", content)
	}
}
