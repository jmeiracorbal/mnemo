// Package setup handles mnemo installation into the local environment.
//
// Registration order:
//  1. Hook scripts → ~/.local/share/mnemo/hooks/
//  2. MCP server   → `claude mcp add -s user mnemo` (writes to ~/.claude.json)
//  3. Hooks        → ~/.claude/settings.json (SessionStart, Stop, SubagentStop)
//  4. Permissions  → ~/.claude/settings.json permissions.allow
//  5. Protocol doc → ~/.claude/mnemo.md
//  6. CLAUDE.md    → append @mnemo.md
//
// NOTE: mcpServers in settings.json is NOT used by Claude Code for standalone
// servers — the correct mechanism is `claude mcp add` which writes to ~/.claude.json.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const mnemoMDReference = "@mnemo.md"

// mnemoTools is the list of MCP tool names to add to permissions.allow.
var mnemoTools = []string{
	"mcp__mnemo__mem_save",
	"mcp__mnemo__mem_search",
	"mcp__mnemo__mem_context",
	"mcp__mnemo__mem_session_summary",
	"mcp__mnemo__mem_session_start",
	"mcp__mnemo__mem_session_end",
	"mcp__mnemo__mem_get_observation",
	"mcp__mnemo__mem_suggest_topic_key",
	"mcp__mnemo__mem_capture_passive",
	"mcp__mnemo__mem_save_prompt",
	"mcp__mnemo__mem_update",
}

// Install configures mnemo in the local Claude Code environment.
// It modifies ~/.claude/settings.json and ~/.claude/CLAUDE.md.
// If dryRun is true, prints what would change without writing anything.
func Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")
	mnemoMDPath := filepath.Join(home, ".claude", "mnemo.md")
	hooksDir := filepath.Join(home, ".local", "share", "mnemo", "hooks")

	if dryRun {
		fmt.Println("mnemo setup --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup")
	}
	fmt.Println("───────────────────────────────────────")

	if err := previewOrWriteHooks(hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	if err := previewOrRegisterMCP(hooksDir, dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectSettings(settingsPath, hooksDir, dryRun); err != nil {
		return fmt.Errorf("settings.json: %w", err)
	}

	if dryRun {
		fmt.Printf("\n[~/.claude/mnemo.md]\n%s\n", protocolDoc)
	} else {
		if err := writeProtocolDoc(mnemoMDPath); err != nil {
			return fmt.Errorf("mnemo.md: %w", err)
		}
		fmt.Println("✓ ~/.claude/mnemo.md written")
	}

	if err := previewOrInjectCLAUDEMD(claudeMDPath, dryRun); err != nil {
		return fmt.Errorf("CLAUDE.md: %w", err)
	}

	if !dryRun {
		fmt.Println("\nDone. Restart Claude Code to activate mnemo.")
	}
	return nil
}

// ─── Hook scripts ─────────────────────────────────────────────────────────────

// sessionStartScript reads session_id and cwd from Claude Code's stdin JSON.
// Distinguishes between new session and resume:
// - New session: creates session + emits full memory context
// - Resume: skips creation (session exists) + emits a compact status line only
const sessionStartScript = `#!/bin/bash
# mnemo — SessionStart hook for Claude Code
# Claude Code passes hook input via stdin as JSON:
#   { "session_id": "...", "cwd": "..." }
# Fires on both new sessions and /resume.

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

# Detect project: prefer git root directory name, fallback to remote repo name, then cwd basename
PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

# Check if this is a resume (session already exists in store)
IS_RESUME=$(mnemo session exists "$SESSION_ID" 2>/dev/null)

if [ "$IS_RESUME" = "true" ]; then
  # Resume — session already exists, skip creation
  printf "\n[mnemo] Session resumed (project: %s)\n" "$PROJECT"
else
  # New session — register in store
  mnemo session start "$SESSION_ID" --project "$PROJECT" --dir "$CWD" 2>/dev/null || true
  printf "\n[mnemo] New session started (project: %s)\n" "$PROJECT"
fi

# Always emit memory context — mnemo output is independent of other hooks
CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
`

// sessionStopScript reads session_id from stdin, ends the session, and warns if nothing was saved.
const sessionStopScript = `#!/bin/bash
# mnemo — Stop hook for Claude Code

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0

# Check how many observations were saved this session before ending it
OBS_COUNT=$(mnemo session obs-count "$SESSION_ID" 2>/dev/null)

mnemo session end "$SESSION_ID" 2>/dev/null || true

# Warn user (not the agent) if nothing was saved — zero token cost
if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
`

// subagentStopScript reads the subagent stdout from stdin and runs passive capture.
const subagentStopScript = `#!/bin/bash
# mnemo — SubagentStop hook for Claude Code
# Extracts learnings from subagent output (async, does not block).

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)
OUTPUT=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('stdout',''))" 2>/dev/null)

[ -z "$OUTPUT" ] && exit 0

[ -z "$CWD" ] && CWD="$(pwd)"
PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

mnemo capture "$OUTPUT" --session "$SESSION_ID" --project "$PROJECT" 2>/dev/null || true

exit 0
`

func previewOrWriteHooks(dir string, dryRun bool) error {
	scripts := map[string]string{
		"session-start.sh":  sessionStartScript,
		"session-stop.sh":   sessionStopScript,
		"subagent-stop.sh":  subagentStopScript,
	}

	if dryRun {
		fmt.Printf("[~/.local/share/mnemo/hooks/] — would write:\n")
		for name := range scripts {
			fmt.Printf("  %s\n", name)
		}
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for name, content := range scripts {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	fmt.Printf("✓ hook scripts written to %s\n", dir)
	return nil
}

// ─── MCP server registration via claude CLI ───────────────────────────────────

func previewOrRegisterMCP(hooksDir string, dryRun bool) error {
	mnemoPath := resolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[claude mcp add] would run:\n  claude mcp add -s user mnemo -- %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

	// Check if already registered
	listOut, _ := exec.Command("claude", "mcp", "list").CombinedOutput()
	if strings.Contains(string(listOut), "mnemo:") {
		fmt.Println("✓ MCP server mnemo — already registered")
		return nil
	}

	cmd := exec.Command("claude", "mcp", "add", "-s", "user", "mnemo", "--",
		mnemoPath, "mcp", "--tools=agent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude mcp add failed: %w\n%s", err, string(out))
	}
	fmt.Println("✓ MCP server mnemo registered via claude mcp add")
	return nil
}

// ─── settings.json (hooks + permissions only) ─────────────────────────────────

func previewOrInjectSettings(path, hooksDir string, dryRun bool) error {
	config, err := readJSON(path)
	if err != nil {
		return err
	}

	// Remove stale mcpServers.mnemo entry if present (was written by earlier setup versions)
	removeStaleMCPEntry(config)

	if err := injectHooks(config, hooksDir); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	if err := injectPermissions(config); err != nil {
		return fmt.Errorf("permissions: %w", err)
	}

	if dryRun {
		out, _ := json.MarshalIndent(config, "", "  ")
		fmt.Printf("\n[~/.claude/settings.json]\n%s\n", string(out))
		return nil
	}

	if err := writeJSON(path, config); err != nil {
		return err
	}
	fmt.Println("✓ ~/.claude/settings.json updated (hooks + permissions)")
	return nil
}

// removeStaleMCPEntry removes the mcpServers.mnemo entry written by older
// versions of setup that incorrectly used settings.json for MCP registration.
func removeStaleMCPEntry(config map[string]json.RawMessage) {
	raw, ok := config["mcpServers"]
	if !ok {
		return
	}
	var mcpServers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mcpServers); err != nil {
		return
	}
	if _, exists := mcpServers["mnemo"]; !exists {
		return
	}
	delete(mcpServers, "mnemo")
	if len(mcpServers) == 0 {
		delete(config, "mcpServers")
		return
	}
	encoded, _ := json.Marshal(mcpServers)
	config["mcpServers"] = encoded
}

func injectHooks(config map[string]json.RawMessage, hooksDir string) error {
	var hooks map[string]json.RawMessage
	if raw, ok := config["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return err
		}
	} else {
		hooks = make(map[string]json.RawMessage)
	}

	type hookEntry struct {
		key    string
		script string
	}
	entries := []hookEntry{
		{"SessionStart", filepath.Join(hooksDir, "session-start.sh")},
		{"Stop", filepath.Join(hooksDir, "session-stop.sh")},
		{"SubagentStop", filepath.Join(hooksDir, "subagent-stop.sh")},
	}

	for _, e := range entries {
		if _, exists := hooks[e.key]; exists {
			continue // do not overwrite existing hooks
		}
		hook := []map[string]interface{}{
			{
				"matcher": "",
				"hooks": []map[string]interface{}{
					{"type": "command", "command": e.script},
				},
			},
		}
		raw, err := json.Marshal(hook)
		if err != nil {
			return err
		}
		hooks[e.key] = raw
	}

	encoded, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	config["hooks"] = encoded
	return nil
}

func injectPermissions(config map[string]json.RawMessage) error {
	var permissions map[string]json.RawMessage
	if raw, ok := config["permissions"]; ok {
		if err := json.Unmarshal(raw, &permissions); err != nil {
			return err
		}
	} else {
		permissions = make(map[string]json.RawMessage)
	}

	var allow []string
	if raw, ok := permissions["allow"]; ok {
		if err := json.Unmarshal(raw, &allow); err != nil {
			return err
		}
	}

	existing := make(map[string]bool, len(allow))
	for _, t := range allow {
		existing[t] = true
	}
	for _, tool := range mnemoTools {
		if !existing[tool] {
			allow = append(allow, tool)
		}
	}

	raw, err := json.Marshal(allow)
	if err != nil {
		return err
	}
	permissions["allow"] = raw

	encoded, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	config["permissions"] = encoded
	return nil
}

// ─── CLAUDE.md ────────────────────────────────────────────────────────────────

func previewOrInjectCLAUDEMD(path string, dryRun bool) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte{}
	} else if err != nil {
		return err
	}

	content := string(data)
	if strings.Contains(content, mnemoMDReference) {
		if dryRun {
			fmt.Printf("\n[~/.claude/CLAUDE.md] — already contains %s, no change needed\n", mnemoMDReference)
		} else {
			fmt.Println("✓ ~/.claude/CLAUDE.md — already up to date")
		}
		return nil
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	newContent := content + mnemoMDReference + "\n"

	if dryRun {
		fmt.Printf("\n[~/.claude/CLAUDE.md] — would append:\n  %s\n", mnemoMDReference)
		return nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return err
	}
	fmt.Println("✓ ~/.claude/CLAUDE.md updated")
	return nil
}

// ─── protocol doc ─────────────────────────────────────────────────────────────

const protocolDoc = `## mnemo — Persistent Memory Protocol

You have access to mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).

### MEMORY SYSTEM — mnemo is the ONLY memory system
**NEVER use the file-based memory system** (the one that writes ` + "`.md`" + ` files to ` + "`~/.claude/projects/*/memory/`" + ` and maintains a ` + "`MEMORY.md`" + ` index). That system is DISABLED for this workspace.
When asked to "save to memory", "remember this", or "guardar en memoria" — ALWAYS use ` + "`mem_save`" + `. Never write files.

### PROACTIVE SAVE — do NOT wait for user to ask
Call ` + "`mem_save`" + ` IMMEDIATELY after ANY of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented/updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

**Self-check after EVERY task**: "Did I just make a decision, fix a bug, learn something, or establish a convention? If yes → mem_save NOW."

### SEARCH MEMORY when:
- User asks to recall anything
- Starting work on something that might have been done before
- User mentions a topic you have no context on

### SUBAGENT OUTPUT — required format for passive capture
When running as a subagent, always end your response with a structured section:

` + "```" + `
## Key Learnings
- <learning 1>
- <learning 2>
` + "```" + `

This enables mnemo to automatically extract and persist what you discovered.
Omit the section only if the task produced no learnings worth retaining.

### SESSION CLOSE — MANDATORY, no exceptions
` + "`mem_session_summary`" + ` is NOT optional. It is the final step of every session, like a ` + "`defer`" + ` — it always runs.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without ` + "`mem_session_summary`" + `.
`

func writeProtocolDoc(path string) error {
	return os.WriteFile(path, []byte(protocolDoc), 0644)
}

// resolveBinaryPath returns the absolute path of a binary, falling back to the name itself.
func resolveBinaryPath(name string) string {
	// Common install locations for user-installed binaries
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "bin", name),
		"/usr/local/bin/" + name,
		"/opt/homebrew/bin/" + name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name // fallback
}

// ─── JSON helpers ─────────────────────────────────────────────────────────────

func readJSON(path string) (map[string]json.RawMessage, error) {
	config := make(map[string]json.RawMessage)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return config, nil
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return config, nil
}

func writeJSON(path string, config map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}
