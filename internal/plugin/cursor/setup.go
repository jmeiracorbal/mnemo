// Package cursor installs mnemo into a Cursor environment.
//
// Registration order:
//  1. Hook scripts → ~/.local/share/mnemo/cursor-hooks/
//  2. MCP server   → ~/.cursor/mcp.json
//  3. Hooks        → ~/.cursor/hooks.json (SessionStart, stop, subagentStop, preCompact)
//  4. Rules doc    → ~/.cursor/rules/mnemo.mdc
//
// NOTE: Script constants below must stay in sync with plugin/cursor/scripts/.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/plugin"
)

const mnemoRulesReference = "mnemo.mdc"

// Installer installs mnemo into a Cursor environment.
type Installer struct{}

// Install configures mnemo in the local Cursor environment.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	hooksDir := filepath.Join(home, ".local", "share", "mnemo", "cursor-hooks")
	cursorHooksJSON := filepath.Join(home, ".cursor", "hooks.json")
	cursorMCPJSON := filepath.Join(home, ".cursor", "mcp.json")
	cursorRulesDir := filepath.Join(home, ".cursor", "rules")

	if dryRun {
		fmt.Println("mnemo setup --cursor --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup --cursor")
	}
	fmt.Println("───────────────────────────────────────")

	if err := previewOrWriteHooks(hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	if err := previewOrRegisterMCP(cursorMCPJSON, dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectHooksJSON(cursorHooksJSON, hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks.json: %w", err)
	}

	if err := previewOrWriteRules(cursorRulesDir, dryRun); err != nil {
		return fmt.Errorf("rules: %w", err)
	}

	if !dryRun {
		fmt.Println("\nDone. Restart Cursor to activate mnemo.")
	}
	return nil
}

// Uninstall is not yet implemented.
func (i Installer) Uninstall() error {
	return fmt.Errorf("uninstall not yet implemented for Cursor")
}

// ─── Hook scripts ─────────────────────────────────────────────────────────────
// Keep in sync with plugin/cursor/scripts/. Tested on Cursor 2.6.21.
//
// Available hooks in Cursor 2.6.x: beforeSubmitPrompt, stop,
// beforeShellExecution, beforeMCPExecution, afterFileEdit.
// sessionStart/sessionEnd/subagentStop/preCompact do NOT exist in v2.6.x.

const beforeSubmitPromptScript = `#!/bin/bash
# mnemo — beforeSubmitPrompt hook for Cursor 2.6+
# Fires before every prompt. First occurrence of a conversation_id creates the session,
# emits general context, and searches for memories relevant to the opening prompt.
# Input: { "conversation_id": "...", "workspace_roots": ["..."], "prompt": "...", "transcript_path": null|"..." }

INPUT=$(cat)
CONVERSATION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('conversation_id',''))" 2>/dev/null)
WORKSPACE=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); roots=d.get('workspace_roots',[]); print(roots[0] if roots else '')" 2>/dev/null)
PROMPT=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('prompt',''))" 2>/dev/null)

[ -z "$CONVERSATION_ID" ] && exit 0
[ -z "$WORKSPACE" ] && WORKSPACE="$(pwd)"

# ─── Ensure ~/.cursor/rules/mnemo.mdc exists ─────────────────────────────────
CURSOR_RULES_DIR="${HOME}/.cursor/rules"
MNEMO_MDC="${CURSOR_RULES_DIR}/mnemo.mdc"

if [ ! -f "$MNEMO_MDC" ]; then
  mkdir -p "$CURSOR_RULES_DIR"
  cat > "$MNEMO_MDC" << 'MDCEOF'
` + rulesDoc + `MDCEOF
fi
# ─────────────────────────────────────────────────────────────────────────────

PROJECT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$WORKSPACE" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE")

IS_KNOWN=$(mnemo session exists "$CONVERSATION_ID" 2>/dev/null)
[ "$IS_KNOWN" = "true" ] && exit 0

mnemo session start "$CONVERSATION_ID" --project "$PROJECT" --dir "$WORKSPACE" 2>/dev/null || true
printf "\n[mnemo] New session started (project: %s)\n" "$PROJECT"

CONTEXT=$(mnemo context "$PROJECT" 2>/dev/null)
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

# Prompt-specific search: only if prompt has meaningful content (>20 chars)
PROMPT_LEN=${#PROMPT}
if [ "$PROMPT_LEN" -gt 20 ]; then
  SEARCH_QUERY=$(echo "$PROMPT" | cut -c1-100)
  SEARCH_RESULTS=$(mnemo search "$SEARCH_QUERY" --project "$PROJECT" --limit 3 2>/dev/null)
  if [ -n "$SEARCH_RESULTS" ] && ! echo "$SEARCH_RESULTS" | grep -q "^No memories found"; then
    printf "\n[mnemo] Relevant memories for this prompt:\n%s\n" "$SEARCH_RESULTS"
  fi
fi

exit 0
`

const stopScript = `#!/bin/bash
# mnemo — stop hook for Cursor 2.6+
# Fires when conversation completes. Reads transcript JSONL for passive capture.
# Input: { "conversation_id": "...", "status": "...", "transcript_path": "...|null", "workspace_roots": ["..."] }

INPUT=$(cat)
CONVERSATION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('conversation_id',''))" 2>/dev/null)
TRANSCRIPT_PATH=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('transcript_path') or '')" 2>/dev/null)
WORKSPACE=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); roots=d.get('workspace_roots',[]); print(roots[0] if roots else '')" 2>/dev/null)

[ -z "$CONVERSATION_ID" ] && exit 0
[ -z "$WORKSPACE" ] && WORKSPACE="$(pwd)"

PROJECT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$WORKSPACE" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE")

if [ -n "$TRANSCRIPT_PATH" ] && [ -f "$TRANSCRIPT_PATH" ]; then
  CONTENT=$(python3 -c "
import sys, json
lines = []
for line in open('$TRANSCRIPT_PATH'):
    try:
        msg = json.loads(line)
        role = msg.get('role','')
        content = msg.get('content','')
        if role == 'assistant' and content:
            if isinstance(content, list):
                for block in content:
                    if isinstance(block, dict) and block.get('type') == 'text':
                        lines.append(block.get('text',''))
            elif isinstance(content, str):
                lines.append(content)
    except:
        pass
print('\n'.join(lines))
" 2>/dev/null)
  if [ -n "$CONTENT" ]; then
    mnemo capture "$CONTENT" --session "$CONVERSATION_ID" --project "$PROJECT" 2>/dev/null || true
  fi
fi

OBS_COUNT=$(mnemo session obs-count "$CONVERSATION_ID" 2>/dev/null)
mnemo session end "$CONVERSATION_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
`

func previewOrWriteHooks(dir string, dryRun bool) error {
	scripts := map[string]string{
		"before-submit-prompt.sh": beforeSubmitPromptScript,
		"stop.sh":                 stopScript,
	}

	if dryRun {
		fmt.Printf("[~/.local/share/mnemo/cursor-hooks/] — would write:\n")
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

// ─── MCP registration via ~/.cursor/mcp.json ─────────────────────────────────

func previewOrRegisterMCP(mcpPath string, dryRun bool) error {
	mnemoPath := plugin.ResolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[~/.cursor/mcp.json] — would add:\n  mnemo: %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

	// Use json.RawMessage to preserve all existing entries unchanged (e.g. url/transport fields).
	var config map[string]map[string]json.RawMessage
	data, err := os.ReadFile(mcpPath)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}
	if config == nil {
		config = make(map[string]map[string]json.RawMessage)
	}
	if config["mcpServers"] == nil {
		config["mcpServers"] = make(map[string]json.RawMessage)
	}

	if _, exists := config["mcpServers"]["mnemo"]; exists {
		fmt.Println("✓ MCP server mnemo — already registered in ~/.cursor/mcp.json")
		return nil
	}

	entry := struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}{Command: mnemoPath, Args: []string{"mcp", "--tools=agent"}}
	raw, _ := json.Marshal(entry)
	config["mcpServers"]["mnemo"] = raw

	if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(mcpPath, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println("✓ MCP server mnemo registered in ~/.cursor/mcp.json")
	return nil
}

// ─── ~/.cursor/hooks.json ─────────────────────────────────────────────────────

type cursorHook struct {
	Command string `json:"command"`
}

type cursorHooksConfig struct {
	Version int                       `json:"version"`
	Hooks   map[string][]cursorHook   `json:"hooks"`
}

func previewOrInjectHooksJSON(path, hooksDir string, dryRun bool) error {
	entries := map[string]string{
		"beforeSubmitPrompt": filepath.Join(hooksDir, "before-submit-prompt.sh"),
		"stop":               filepath.Join(hooksDir, "stop.sh"),
	}

	if dryRun {
		fmt.Printf("\n[~/.cursor/hooks.json] — would add hooks:\n")
		for event, script := range entries {
			fmt.Printf("  %s: %s\n", event, script)
		}
		return nil
	}

	config := cursorHooksConfig{Version: 1}
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}
	if config.Hooks == nil {
		config.Hooks = make(map[string][]cursorHook)
	}
	if config.Version == 0 {
		config.Version = 1
	}

	added := 0
	for event, script := range entries {
		if hookExists(config.Hooks[event], script) {
			continue
		}
		config.Hooks[event] = append(config.Hooks[event], cursorHook{Command: script})
		added++
	}

	if added == 0 {
		fmt.Println("✓ ~/.cursor/hooks.json — already up to date")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ ~/.cursor/hooks.json updated (%d hooks added)\n", added)
	return nil
}

func hookExists(hooks []cursorHook, command string) bool {
	for _, h := range hooks {
		if strings.EqualFold(h.Command, command) {
			return true
		}
	}
	return false
}

// ─── ~/.cursor/rules/mnemo.mdc ────────────────────────────────────────────────

const rulesDoc = `---
description: mnemo persistent memory protocol
alwaysApply: true
---

## mnemo — Persistent Memory Protocol

You have access to mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).

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

Omit the section only if the task produced no learnings worth retaining.

### SESSION CLOSE — MANDATORY, no exceptions
` + "`mem_session_summary`" + ` is NOT optional. It is the final step of every session.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without ` + "`mem_session_summary`" + `.
`

func previewOrWriteRules(dir string, dryRun bool) error {
	rulesPath := filepath.Join(dir, mnemoRulesReference)

	if dryRun {
		fmt.Printf("\n[~/.cursor/rules/mnemo.mdc] — would write protocol doc\n")
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(rulesPath, []byte(rulesDoc), 0644); err != nil {
		return err
	}
	fmt.Println("✓ ~/.cursor/rules/mnemo.mdc written")
	return nil
}
