package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/doctor"
)

// installFakeCLI puts an executable named name on PATH that runs script.
func installFakeCLI(t *testing.T, name, script string) {
	t.Helper()
	bin := t.TempDir()
	path := filepath.Join(bin, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func checkByID(report doctor.Report, id string) doctor.Check {
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	return doctor.Check{}
}

func TestDoctorFixRemovesLegacyClaudePluginAndRestoresSetup(t *testing.T) {
	home := t.TempDir()
	plugins := filepath.Join(home, ".claude", "plugins")
	writeFile(t, filepath.Join(plugins, "installed_plugins.json"), `{"version":2,"plugins":{"mnemo@mnemo":[{"scope":"user","installPath":"/x"}]}}`)
	writeFile(t, filepath.Join(plugins, "known_marketplaces.json"), `{"mnemo":{}}`)
	// The fake CLI applies the registry changes the real one makes.
	log := filepath.Join(home, "claude.log")
	installFakeCLI(t, "claude", `echo "$*" >> '`+log+`'
case "$*" in
  "plugin uninstall mnemo@mnemo --scope user") echo '{"version":2,"plugins":{}}' > '`+plugins+`/installed_plugins.json' ;;
  "plugin marketplace remove mnemo") echo '{}' > '`+plugins+`/known_marketplaces.json' ;;
esac
`)
	opts := doctorOptions{Agent: "claudecode", Path: home, Home: home, DataDir: filepath.Join(home, ".mnemo")}

	before := doctor.BuildReport(toDoctorOptions(opts))
	legacy := checkByID(before, "legacy_plugin.claudecode")
	if legacy.Status != "warning" || legacy.Fix != "mnemo doctor --fix --agent=claudecode" {
		t.Fatalf("legacy check before fix = %+v", legacy)
	}
	if hooks := checkByID(before, "runtime_files.claudecode"); hooks.Status == "ok" || hooks.Fix == "" {
		t.Fatalf("hooks check before fix = %+v, want failing with a fix", hooks)
	}

	var out bytes.Buffer
	if err := fixDoctorReport(before, opts, &out); err != nil {
		t.Fatalf("fix: %v\n%s", err, out.String())
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(calls)); got != "plugin uninstall mnemo@mnemo --scope user\nplugin marketplace remove mnemo" {
		t.Fatalf("claude CLI calls = %q", got)
	}

	after := doctor.BuildReport(toDoctorOptions(opts))
	for _, id := range []string{"legacy_plugin.claudecode", "runtime_files.claudecode", "mcp_config.claudecode", "global_instructions.claudecode"} {
		if check := checkByID(after, id); check.Status != "ok" {
			t.Errorf("%s after fix = %+v, want ok", id, check)
		}
	}

	out.Reset()
	if err := fixDoctorReport(after, opts, &out); err != nil || !strings.Contains(out.String(), "nothing to repair") {
		t.Fatalf("second fix err=%v out=%q", err, out.String())
	}
}

func TestClaudePreToolUseHookDeniesWritesWithoutMarker(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(self, filepath.Join(bin, "mnemo")); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "claudecode", "hooks", "pre-tool-use.sh"))
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	run := func(input string) string {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Dir = project
		cmd.Stdin = strings.NewReader(input)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "MNEMO_TEST_RUN_MAIN=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("hook failed: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	call := func(tool, projectID string) string {
		input, _ := json.Marshal(map[string]any{"tool_name": tool, "cwd": project, "tool_input": map[string]any{"project": projectID}})
		return string(input)
	}
	deny := func(output string) string {
		t.Helper()
		var decision struct {
			HookSpecificOutput struct {
				HookEventName            string `json:"hookEventName"`
				PermissionDecision       string `json:"permissionDecision"`
				PermissionDecisionReason string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(output), &decision); err != nil {
			t.Fatalf("hook output is not JSON: %q", output)
		}
		if decision.HookSpecificOutput.HookEventName != "PreToolUse" || decision.HookSpecificOutput.PermissionDecision != "deny" {
			t.Fatalf("hook output = %q, want a PreToolUse deny", output)
		}
		return decision.HookSpecificOutput.PermissionDecisionReason
	}

	if reason := deny(run(call("mcp__mnemo__mem_save", "p1"))); !strings.Contains(reason, "not initialized") {
		t.Fatalf("reason = %q", reason)
	}
	if out := run(call("mcp__mnemo__mem_search", "p1")); out != "" {
		t.Fatalf("read tool output = %q, want none", out)
	}
	writeFile(t, filepath.Join(project, ".mnemo"), `{"version":1,"id":"p1"}`)
	if out := run(call("mcp__mnemo__mem_save", "p1")); out != "" {
		t.Fatalf("matching project output = %q, want none", out)
	}
	if reason := deny(run(call("mcp__mnemo__mem_save", "p2"))); !strings.Contains(reason, "mismatch") {
		t.Fatalf("reason = %q", reason)
	}
}
