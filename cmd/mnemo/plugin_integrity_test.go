package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestShippedHooksReferenceRealScripts validates that every command in
// plugin/claude-code/hooks/hooks.json points to a script that actually
// exists in plugin/claude-code/scripts/. This catches filename mismatches
// before they reach a published release.
func TestShippedHooksReferenceRealScripts(t *testing.T) {
	hooksFile := filepath.Join("..", "..", "plugin", "claude-code", "hooks", "hooks.json")
	data, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("could not read shipped hooks.json: %v", err)
	}

	var raw struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("could not parse hooks.json: %v", err)
	}

	const prefix = "${CLAUDE_PLUGIN_ROOT}/scripts/"
	scriptsDir := filepath.Join("..", "..", "plugin", "claude-code", "scripts")
	validatedCount := 0

	for event, matchers := range raw.Hooks {
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				cmd := hook.Command
				// Search all tokens so commands like "bash ${CLAUDE_PLUGIN_ROOT}/scripts/foo.sh"
				// are also validated, not only commands that start with the prefix.
				var scriptName string
				for _, tok := range strings.Fields(cmd) {
					tok = strings.Trim(tok, `"'`)
					if strings.HasPrefix(tok, prefix) {
						scriptName = strings.TrimPrefix(tok, prefix)
						break
					}
				}
				if scriptName == "" {
					continue
				}
				validatedCount++
				scriptName = filepath.Clean(scriptName)
				if filepath.IsAbs(scriptName) || scriptName == ".." || strings.HasPrefix(scriptName, ".."+string(os.PathSeparator)) {
					t.Errorf("hooks.json [%s]: invalid script path %q", event, scriptName)
					continue
				}
				scriptPath := filepath.Join(scriptsDir, scriptName)
				if _, err := os.Stat(scriptPath); err != nil {
					t.Errorf("hooks.json [%s]: references %q but file does not exist at %s", event, scriptName, scriptPath)
				}
			}
		}
	}
	if validatedCount == 0 {
		t.Fatalf("no script references were validated from hooks.json")
	}
}

func TestShippedHooksResolveProjectFromMarker(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "plugin", "claude-code", "scripts"),
		filepath.Join("..", "..", "scripts", "cursor", "hooks"),
		filepath.Join("..", "..", "scripts", "windsurf", "hooks"),
		filepath.Join("..", "..", "scripts", "codex", "hooks"),
		filepath.Join("..", "..", "scripts", "opencode", "plugins"),
	}

	checked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".sh", ".ts":
			default:
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read %s: %v", path, err)
				return nil
			}
			content := string(data)
			usesProject := strings.Contains(content, "$PROJECT") ||
				strings.Contains(content, "PROJECT=") ||
				strings.Contains(content, "--project") ||
				strings.Contains(content, "mnemoProject(")
			if !usesProject {
				return nil
			}
			checked++

			if !strings.Contains(content, ".mnemo") {
				t.Errorf("%s uses project identity but does not read .mnemo", path)
			}
			fromMarker := strings.Contains(content, "mnemo json id") ||
				strings.Contains(content, ".id")
			if !fromMarker {
				t.Errorf("%s uses project identity but does not resolve it from .mnemo id", path)
			}
			if strings.Contains(content, "tr '/' '-'") || strings.Contains(content, `tr "/" "-"`) {
				t.Errorf("%s still derives project identity from filesystem path", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if checked == 0 {
		t.Fatal("no shipped hooks that resolve PROJECT were found")
	}
}

func TestShippedProtocolsForbidFallbackMemory(t *testing.T) {
	files := []string{
		filepath.Join("..", "..", "templates", "rules", "generic.md"),
		filepath.Join("..", "..", "templates", "rules", "global.md"),
		filepath.Join("..", "..", "templates", "rules", "cursor.mdc"),
		filepath.Join("..", "..", "templates", "rules", "cursor-global.mdc"),
		filepath.Join("..", "..", "templates", "rules", "windsurf.md"),
		filepath.Join("..", "..", "plugin", "claude-code", "scripts", "mnemo.md"),
		filepath.Join("..", "..", "plugin", "claude-code", "scripts", "session-start-protocol.md"),
		filepath.Join("..", "..", "plugin", "claude-code", "scripts", "post-compact-protocol-header.md"),
		filepath.Join("..", "..", "scripts", "codex", "hooks", "mnemo-protocol.md"),
		filepath.Join("..", "..", "scripts", "cursor", "rules", "mnemo.mdc"),
		filepath.Join("..", "..", "scripts", "windsurf", "templates", "global_rules.md"),
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "mnemo is the ONLY persistent memory system") {
			t.Errorf("%s does not declare mnemo as the only persistent memory", path)
		}
		if !strings.Contains(content, "plaintext files as a memory fallback") {
			t.Errorf("%s does not forbid plaintext memory fallback", path)
		}
	}
}

func TestShippedMnemoMemorySkill(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "mnemo-memory", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped skill: %v", err)
	}
	content := string(data)
	required := []string{
		"name: mnemo-memory",
		"description:",
		"<root>/.mnemo",
		"non-empty `id`",
		"Never create `MEMORY.md`",
		"`mem_session_summary`",
		"mnemo doctor",
	}
	for _, value := range required {
		if !strings.Contains(content, value) {
			t.Errorf("shipped skill missing %q", value)
		}
	}
}

func TestShippedMnemoProjectMaintenanceSkill(t *testing.T) {
	skillPath := filepath.Join("..", "..", "skills", "mnemo-project-maintenance", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read shipped project maintenance skill: %v", err)
	}
	content := string(data)
	required := []string{
		"name: mnemo-project-maintenance",
		"description:",
		"mnemo projects list --json",
		"mnemo projects merge --auto-by-path --dry-run --json",
		"Do not run `mnemo projects merge ... --yes` unless the user explicitly approves",
		"`from` is the duplicate/source project",
		"`to` is the canonical destination project",
	}
	for _, value := range required {
		if !strings.Contains(content, value) {
			t.Errorf("shipped project maintenance skill missing %q", value)
		}
	}
}

func TestShippedMnemoMemoryCurationSkill(t *testing.T) {
	skillPath := filepath.Join("..", "..", "skills", "mnemo-memory-curation", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read shipped memory curation skill: %v", err)
	}
	content := string(data)
	required := []string{
		"name: mnemo-memory-curation",
		"description:",
		"mnemo memories review --json",
		"mnemo memories supersede OLD_ID --by=NEW_ID --reason=TEXT",
		"Notify the user when likely conflicts are found during normal work",
		"Never edit `~/.mnemo/memory.db`",
		"mem_save",
	}
	for _, value := range required {
		if !strings.Contains(content, value) {
			t.Errorf("shipped memory curation skill missing %q", value)
		}
	}
}

type shippedSkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type shippedSkillOpenAIInterface struct {
	DisplayName      string `yaml:"display_name"`
	ShortDescription string `yaml:"short_description"`
	DefaultPrompt    string `yaml:"default_prompt"`
}

type shippedSkillOpenAIMetadata struct {
	Interface shippedSkillOpenAIInterface `yaml:"interface"`
}

func TestShippedSkillsHaveOpenAIMetadata(t *testing.T) {
	skillsRoot := filepath.Join("..", "..", "skills")
	wantMetadata := map[string]shippedSkillOpenAIInterface{
		"mnemo-memory": {
			DisplayName:      "mnemo Memory",
			ShortDescription: "Use persistent memory safely across sessions",
			DefaultPrompt:    "Use $mnemo-memory to recover relevant context and persist important learnings for this project.",
		},
		"mnemo-project-maintenance": {
			DisplayName:      "mnemo Project Maintenance",
			ShortDescription: "Detect and safely merge duplicate projects",
			DefaultPrompt:    "Use $mnemo-project-maintenance to inspect mnemo project inventory, propose duplicate project merges, and apply only explicitly approved repairs.",
		},
		"mnemo-memory-curation": {
			DisplayName:      "mnemo Memory Curation",
			ShortDescription: "Detect and safely repair memory conflicts",
			DefaultPrompt:    "Use $mnemo-memory-curation to proactively review mnemo memory conflicts, warn the user, and apply only explicitly approved repairs.",
		},
	}

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read shipped skills: %v", err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			t.Errorf("shipped skill %s must be a real directory, not a symlink", entry.Name())
			continue
		}
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		seen[skillName] = true
		want, ok := wantMetadata[skillName]
		if !ok {
			t.Errorf("shipped skill %s missing expected OpenAI metadata contract", skillName)
			continue
		}

		t.Run(skillName, func(t *testing.T) {
			skillDir := filepath.Join(skillsRoot, skillName)
			frontmatter := readShippedSkillFrontmatter(t, filepath.Join(skillDir, "SKILL.md"))
			if frontmatter.Name != skillName {
				t.Fatalf("SKILL.md name = %q, want %q", frontmatter.Name, skillName)
			}
			if strings.TrimSpace(frontmatter.Description) == "" {
				t.Fatalf("SKILL.md description is empty")
			}

			metadata := readShippedOpenAIMetadata(t, filepath.Join(skillDir, "agents", "openai.yaml"))
			assertOpenAIInterface(t, metadata.Interface, want, skillName)
		})
	}
	if len(seen) == 0 {
		t.Fatalf("no shipped skills found")
	}
	for skillName := range wantMetadata {
		if !seen[skillName] {
			t.Errorf("expected shipped skill %s was not found", skillName)
		}
	}
}

func TestCutShippedSkillFrontmatterRequiresDelimiterLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantOK  bool
	}{
		{
			name:    "complete delimiter line before body",
			content: "---\nname: mnemo-memory\n---\n# Body\n",
			want:    "name: mnemo-memory",
			wantOK:  true,
		},
		{
			name:    "complete delimiter line at EOF",
			content: "---\nname: mnemo-memory\n---",
			want:    "name: mnemo-memory",
			wantOK:  true,
		},
		{
			name:    "reject delimiter suffix",
			content: "---\nname: mnemo-memory\n---invalid\n# Body\n",
			wantOK:  false,
		},
		{
			name:    "missing delimiter",
			content: "---\nname: mnemo-memory\n# Body\n",
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cutShippedSkillFrontmatter(tt.content)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("frontmatter = %q, want %q", got, tt.want)
			}
		})
	}
}

func readShippedSkillFrontmatter(t *testing.T, path string) shippedSkillFrontmatter {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped skill frontmatter: %v", err)
	}
	frontmatterText, ok := cutShippedSkillFrontmatter(string(data))
	if !ok {
		t.Fatalf("%s has malformed YAML frontmatter", path)
	}
	var frontmatter shippedSkillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &frontmatter); err != nil {
		t.Fatalf("parse %s frontmatter: %v", path, err)
	}
	return frontmatter
}

func cutShippedSkillFrontmatter(content string) (string, bool) {
	const openingDelimiter = "---\n"
	if !strings.HasPrefix(content, openingDelimiter) {
		return "", false
	}
	rest := strings.TrimPrefix(content, openingDelimiter)
	if frontmatterText, _, ok := strings.Cut(rest, "\n---\n"); ok {
		return frontmatterText, true
	}
	frontmatterText, ok := strings.CutSuffix(rest, "\n---")
	if !ok {
		return "", false
	}
	return frontmatterText, true
}

func readShippedOpenAIMetadata(t *testing.T, path string) shippedSkillOpenAIMetadata {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped skill OpenAI metadata: %v", err)
	}
	var metadata shippedSkillOpenAIMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("parse shipped skill OpenAI metadata: %v", err)
	}
	return metadata
}

func assertOpenAIInterface(t *testing.T, got, want shippedSkillOpenAIInterface, skillName string) {
	t.Helper()
	gotFields := map[string]string{
		"interface.display_name":      got.DisplayName,
		"interface.short_description": got.ShortDescription,
		"interface.default_prompt":    got.DefaultPrompt,
	}
	wantFields := map[string]string{
		"interface.display_name":      want.DisplayName,
		"interface.short_description": want.ShortDescription,
		"interface.default_prompt":    want.DefaultPrompt,
	}
	for key, want := range wantFields {
		if got := gotFields[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if !strings.Contains(got.DefaultPrompt, "$"+skillName) {
		t.Errorf("interface.default_prompt must mention $%s", skillName)
	}
	if l := len(got.ShortDescription); l < 25 || l > 64 {
		t.Errorf("interface.short_description length = %d, want 25..64", l)
	}
}
