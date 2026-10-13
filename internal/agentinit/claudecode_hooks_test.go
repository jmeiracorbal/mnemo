package agentinit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestClaudeCodeHooksRunInstalledExecutableScripts(t *testing.T) {
	home := t.TempDir()
	modes := map[string]os.FileMode{}
	for _, target := range claudeCodeRuntimeAssets(home) {
		if !strings.HasPrefix(target.Path, claudeCodeHooksDir(home)+string(filepath.Separator)) {
			t.Fatalf("asset %s installs outside the owned hooks dir", target.Path)
		}
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(target.Asset))); err != nil {
			t.Fatalf("embedded asset %s missing: %v", target.Asset, err)
		}
		modes[target.Path] = target.Mode
	}
	referenced := map[string]bool{}
	for event, groups := range claudeCodeHooks(home) {
		for _, group := range groups {
			for _, h := range group.(map[string]any)["hooks"].([]any) {
				hook := h.(map[string]any)
				command := hook["command"].(string)
				if _, ok := hook["args"]; !ok {
					t.Errorf("%s hook %s must use exec form", event, command)
				}
				if modes[command] != 0755 {
					t.Errorf("%s hook %s is not an executable runtime asset", event, command)
				}
				referenced[command] = true
			}
		}
	}
	for path, mode := range modes {
		if mode == 0755 && !referenced[path] {
			t.Errorf("executable asset %s is not registered as a hook", path)
		}
	}
}

func TestRefreshRegistersClaudeCodeHooksWithoutTouchingUserSettings(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, ".claude", "settings.json")
	writeTestFile(t, settings, `{"theme":"dark","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/guard"}]}],"Stop":[{"hooks":[{"type":"command","command":"/usr/local/bin/notify"}]}]}}`)

	if _, err := Refresh(home, "/bin/mnemo", AgentClaudeCode); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	root := readSettings(t, settings)
	if root["theme"] != "dark" {
		t.Fatalf("user setting lost: %v", root)
	}
	commands := hookCommands(root)
	for _, want := range []string{"/usr/local/bin/guard", "/usr/local/bin/notify", filepath.Join(claudeCodeHooksDir(home), "pre-tool-use.sh"), filepath.Join(claudeCodeHooksDir(home), "session-start.sh")} {
		if !slices.Contains(commands, want) {
			t.Fatalf("hook commands %v missing %s", commands, want)
		}
	}
	for _, target := range claudeCodeRuntimeAssets(home) {
		info, err := os.Stat(target.Path)
		if err != nil || info.Mode().Perm() != target.Mode {
			t.Fatalf("runtime asset %s: info=%v err=%v, want mode %v", target.Path, info, err, target.Mode)
		}
	}
	if check := claudeCodeCheckRuntime(home); check.Status != "ok" {
		t.Fatalf("runtime check = %+v, want ok", check)
	}

	if _, err := Uninstall(home, AgentClaudeCode); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	root = readSettings(t, settings)
	if got := hookCommands(root); !slices.Equal(got, []string{"/usr/local/bin/guard", "/usr/local/bin/notify"}) {
		t.Fatalf("hook commands after uninstall = %v, want only the user's", got)
	}
	if root["theme"] != "dark" {
		t.Fatalf("user setting lost on uninstall: %v", root)
	}
}

func TestUpsertOwnedHooksReplacesStaleEntries(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	owned := filepath.Join(dir, "hooks", "mnemo")
	stale := `{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"` + filepath.Join(owned, "old.sh") + `","timeout":1}]}],"PostCompact":[{"hooks":[{"type":"command","command":"` + filepath.Join(owned, "post-compact.sh") + `"}]}]}}`
	writeTestFile(t, settings, stale)
	hooks := map[string][]any{"SessionStart": {map[string]any{"matcher": "startup", "hooks": []any{map[string]any{"type": "command", "command": filepath.Join(owned, "new.sh")}}}}}

	changed, err := upsertOwnedHooks(settings, owned, hooks)
	if err != nil || !changed {
		t.Fatalf("upsert changed=%v err=%v, want true/nil", changed, err)
	}
	root := readSettings(t, settings)
	if got := hookCommands(root); !slices.Equal(got, []string{filepath.Join(owned, "new.sh")}) {
		t.Fatalf("hook commands = %v, want only the new owned hook", got)
	}
	if _, ok := root["hooks"].(map[string]any)["PostCompact"]; ok {
		t.Fatal("event left without hooks was not removed")
	}
	info, _ := os.Stat(settings)
	changed, err = upsertOwnedHooks(settings, owned, hooks)
	if err != nil || changed {
		t.Fatalf("second upsert changed=%v err=%v, want false/nil", changed, err)
	}
	if after, _ := os.Stat(settings); !after.ModTime().Equal(info.ModTime()) {
		t.Fatal("no-op upsert rewrote settings")
	}

	missing := filepath.Join(dir, "absent.json")
	if changed, err := upsertOwnedHooks(missing, owned, nil); err != nil || changed {
		t.Fatalf("removing from a missing file changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("removing hooks created a settings file")
	}
}

func TestClaudeCodeLegacyPluginDetectionAndCommands(t *testing.T) {
	home := t.TempDir()
	if check := claudeCodeCheckLegacy(home); check.Status != "ok" {
		t.Fatalf("clean install check = %+v, want ok", check)
	}
	plugins := filepath.Join(home, ".claude", "plugins")
	writeTestFile(t, filepath.Join(plugins, "installed_plugins.json"), `{"version":2,"plugins":{"mnemo@mnemo":[{"scope":"user","installPath":"/x"},{"scope":"project","projectPath":"/work/app","installPath":"/y"}],"other@x":[{"scope":"user"}]}}`)
	writeTestFile(t, filepath.Join(plugins, "known_marketplaces.json"), `{"mnemo":{},"other":{}}`)

	check := claudeCodeCheckLegacy(home)
	if check.Status != "warning" || check.Fix != "mnemo doctor --fix --agent=claudecode" {
		t.Fatalf("legacy check = %+v", check)
	}
	want := "claude plugin uninstall mnemo@mnemo --scope user && cd '/work/app' && claude plugin uninstall mnemo@mnemo --scope project && claude plugin marketplace remove mnemo"
	if got := check.Details["commands"]; got != want {
		t.Fatalf("commands = %q\nwant      %q", got, want)
	}
}

func TestCodexLegacyPluginDetectionAndCommands(t *testing.T) {
	home := t.TempDir()
	if check := codexCheckLegacy(home); check.Status != "ok" {
		t.Fatalf("clean install check = %+v, want ok", check)
	}
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), "model = \"gpt\"\n\n[marketplaces.mnemo]\nsource_type = \"git\"\n\n[plugins.\"mnemo@mnemo\"]\nenabled = true\n")
	check := codexCheckLegacy(home)
	if check.Status != "warning" || check.Details["commands"] != "codex plugin remove mnemo@mnemo && codex plugin marketplace remove mnemo" {
		t.Fatalf("legacy check = %+v", check)
	}
}

func TestRunLegacyCommandsUsesAgentCLI(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(home, "calls.log")
	writeTestFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\necho \"$PWD|$HOME|$*\" >> '"+log+"'\n")
	if err := os.Chmod(filepath.Join(bin, "claude"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	project := t.TempDir()

	ran, err := runLegacyCommands(home, []legacyCommand{
		{Name: "claude", Args: []string{"plugin", "uninstall", "mnemo@mnemo", "--scope", "project"}, Dir: project},
		{Name: "claude", Args: []string{"plugin", "marketplace", "remove", "mnemo"}},
	})
	if err != nil || len(ran) != 2 {
		t.Fatalf("run legacy commands ran=%v err=%v", ran, err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	project, _ = filepath.EvalSymlinks(project)
	if !strings.HasPrefix(lines[0], project+"|"+home+"|plugin uninstall mnemo@mnemo --scope project") {
		t.Fatalf("first call = %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], "|"+home+"|plugin marketplace remove mnemo") {
		t.Fatalf("second call = %q", lines[1])
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := runLegacyCommands(home, []legacyCommand{{Name: "codex", Args: []string{"plugin", "remove", "mnemo@mnemo"}}}); err == nil || !strings.Contains(err.Error(), "run manually: codex plugin remove mnemo@mnemo") {
		t.Fatalf("missing CLI error = %v", err)
	}
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return root
}

// hookCommands lists every hook command in settings, sorted.
func hookCommands(root map[string]any) []string {
	var commands []string
	hooks, _ := root["hooks"].(map[string]any)
	for _, groups := range hooks {
		for _, group := range groups.([]any) {
			for _, h := range group.(map[string]any)["hooks"].([]any) {
				if command, ok := h.(map[string]any)["command"].(string); ok && !slices.Contains(commands, command) {
					commands = append(commands, command)
				}
			}
		}
	}
	slices.Sort(commands)
	return commands
}
