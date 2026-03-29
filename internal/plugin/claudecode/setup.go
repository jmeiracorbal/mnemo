// Package claudecode installs mnemo into a Claude Code environment.
//
// Registration order:
//  1. Hook scripts → ~/.local/share/mnemo/hooks/
//  2. MCP server   → `claude mcp add -s user mnemo` (writes to ~/.claude.json)
//  3. Hooks        → ~/.claude/settings.json (SessionStart, Stop, SubagentStop)
//  4. Permissions  → ~/.claude/settings.json permissions.allow
//  5. Protocol doc → ~/.claude/mnemo.md
//  6. CLAUDE.md    → append @mnemo.md
//
// NOTE: Script constants below must stay in sync with plugin/claude-code/scripts/.
// NOTE: mcpServers in settings.json is NOT used by Claude Code for standalone
// servers — the correct mechanism is `claude mcp add` which writes to ~/.claude.json.
package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/plugin"
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

// Installer installs mnemo into a Claude Code environment.
type Installer struct{}

// Install configures mnemo in the local Claude Code environment.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
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

	if err := previewOrRegisterMCP(dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectSettings(settingsPath, hooksDir, dryRun); err != nil {
		return fmt.Errorf("settings.json: %w", err)
	}

	if dryRun {
		fmt.Printf("\n[~/.claude/mnemo.md]\n%s\n", protocolDoc)
	} else {
		if err := os.WriteFile(mnemoMDPath, []byte(protocolDoc), 0644); err != nil {
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

// Uninstall is not yet implemented.
func (i Installer) Uninstall() error {
	return fmt.Errorf("uninstall not yet implemented for Claude Code")
}

// ─── Hook scripts ─────────────────────────────────────────────────────────────
// Keep in sync with plugin/claude-code/scripts/.

const sessionStartScript = `#!/bin/bash
# mnemo — SessionStart hook for Claude Code plugin
# Claude Code passes hook input via stdin as JSON:
#   { "session_id": "...", "cwd": "..." }

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

# Detect project: prefer git root directory name, fallback to remote repo name, then cwd basename
PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

IS_RESUME=$(mnemo session exists "$SESSION_ID" 2>/dev/null)

if [ "$IS_RESUME" = "true" ]; then
  printf "\n[mnemo] Session resumed (project: %s)\n" "$PROJECT"
else
  mnemo session start "$SESSION_ID" --project "$PROJECT" --dir "$CWD" 2>/dev/null || true
  printf "\n[mnemo] New session started (project: %s)\n" "$PROJECT"
fi

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

cat <<'PROTOCOL'

### FIRST ACTION — load memory tools
Memory tools are deferred and must be loaded before use. Call ToolSearch NOW with:
select:mcp__mnemo__mem_save,mcp__mnemo__mem_context,mcp__mnemo__mem_search,mcp__mnemo__mem_session_summary,mcp__mnemo__mem_session_end
PROTOCOL

exit 0
`

const sessionStopScript = `#!/bin/bash
# mnemo — Stop hook for Claude Code plugin

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0

OBS_COUNT=$(mnemo session obs-count "$SESSION_ID" 2>/dev/null)

mnemo session end "$SESSION_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
`

const subagentStopScript = `#!/bin/bash
# mnemo — SubagentStop hook for Claude Code plugin
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

const postCompactResumeScript = `#!/bin/bash
# mnemo — SessionStart (source: compact) hook for Claude Code plugin
# Fires when a new context window starts after compaction.
# The compacted summary was already persisted by the PostCompact hook.

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

printf "\n[mnemo] Context restored after compaction (project: %s)\n" "$PROJECT"

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)

cat <<'PROTOCOL'

### FIRST ACTION — load memory tools
Memory tools are deferred and must be loaded before use. Call ToolSearch NOW with:
select:mcp__mnemo__mem_save,mcp__mnemo__mem_context,mcp__mnemo__mem_search,mcp__mnemo__mem_session_summary,mcp__mnemo__mem_session_end

### POST-COMPACTION RECOVERY — context window was just reset
The compacted summary above contains what happened before compaction.
The summary was already persisted to mnemo by the PostCompact hook.

Recovery steps (do BEFORE responding to user):
1. Call mem_context with the current project to recover recent session history
2. If you need detail on a specific topic, call mem_search with relevant keywords
3. Only THEN continue working on what the user asked
PROTOCOL

if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
`

const postCompactionScript = `#!/bin/bash
# mnemo — PostCompact hook for Claude Code plugin
# Injects memory protocol and context after compaction so the agent
# persists the compacted summary and recovers session state.

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$CWD" ] && CWD="$(pwd)"

PROJECT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$CWD" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$CWD")

# Ensure session exists
if [ -n "$SESSION_ID" ] && [ -n "$PROJECT" ]; then
  mnemo session start "$SESSION_ID" --project "$PROJECT" --dir "$CWD" 2>/dev/null || true
fi

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)

cat <<'PROTOCOL'
## mnemo Persistent Memory — ACTIVE PROTOCOL

You have mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).
This protocol is MANDATORY and ALWAYS ACTIVE.

### PROACTIVE SAVE — do NOT wait for user to ask
Call ` + "`mem_save`" + ` IMMEDIATELY after ANY of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented/updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

### SESSION CLOSE — before saying "done"/"listo":
Call ` + "`mem_session_summary`" + ` with: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

---

CRITICAL INSTRUCTION POST-COMPACTION — follow these steps IN ORDER:
PROTOCOL

printf "\n1. FIRST: Call mem_session_summary with the content of the compacted summary above. Use project: '%s'.\n" "$PROJECT"
printf "   This preserves what was accomplished before compaction.\n\n"
printf "2. THEN: Call mem_context with project: '%s' to recover recent session history and observations.\n" "$PROJECT"
printf "   Read the returned context carefully — it tells you what was being worked on.\n\n"

cat <<'PROTOCOL'
3. If you need more detail on a specific topic, call mem_search with relevant keywords.

4. Only THEN continue working on what the user asked.

All 4 steps are MANDATORY. Without them, you lose context and start blind.
PROTOCOL

if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
`

// protocolDoc is written to ~/.claude/mnemo.md during install.
// Keep in sync with plugin/templates/memory/mnemo.md.
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

func previewOrWriteHooks(dir string, dryRun bool) error {
	scripts := map[string]string{
		"session-start.sh":       sessionStartScript,
		"session-stop.sh":        sessionStopScript,
		"subagent-stop.sh":       subagentStopScript,
		"post-compaction.sh":     postCompactionScript,
		"post-compact-resume.sh": postCompactResumeScript,
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

func previewOrRegisterMCP(dryRun bool) error {
	mnemoPath := plugin.ResolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[claude mcp add] would run:\n  claude mcp add -s user mnemo -- %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

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
	config, err := plugin.ReadJSON(path)
	if err != nil {
		return err
	}

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

	if err := plugin.WriteJSON(path, config); err != nil {
		return err
	}
	fmt.Println("✓ ~/.claude/settings.json updated (hooks + permissions)")
	return nil
}

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

	sessionStartScript := filepath.Join(hooksDir, "session-start.sh")
	postCompactResumeScript := filepath.Join(hooksDir, "post-compact-resume.sh")
	postCompactionScript := filepath.Join(hooksDir, "post-compaction.sh")

	// SessionStart: separate matchers per source so compact gets its own recovery script.
	if _, exists := hooks["SessionStart"]; !exists {
		sessionStartHook := []map[string]interface{}{
			{"matcher": "startup", "hooks": []map[string]interface{}{{"type": "command", "command": sessionStartScript}}},
			{"matcher": "resume", "hooks": []map[string]interface{}{{"type": "command", "command": sessionStartScript}}},
			{"matcher": "clear", "hooks": []map[string]interface{}{{"type": "command", "command": sessionStartScript}}},
			{"matcher": "compact", "hooks": []map[string]interface{}{{"type": "command", "command": postCompactResumeScript}}},
		}
		raw, err := json.Marshal(sessionStartHook)
		if err != nil {
			return err
		}
		hooks["SessionStart"] = raw
	}

	type hookEntry struct {
		key    string
		script string
	}
	simpleEntries := []hookEntry{
		{"Stop", filepath.Join(hooksDir, "session-stop.sh")},
		{"SubagentStop", filepath.Join(hooksDir, "subagent-stop.sh")},
		{"PostCompact", postCompactionScript},
	}

	for _, e := range simpleEntries {
		if _, exists := hooks[e.key]; exists {
			continue
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
