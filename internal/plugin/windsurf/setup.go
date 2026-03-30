// Package windsurf installs mnemo into a Windsurf environment.
//
// Registration order:
//  1. Hook scripts → ~/.local/share/mnemo/windsurf-hooks/
//  2. MCP server   → ~/.codeium/windsurf/mcp_config.json
//  3. Hooks        → ~/.codeium/windsurf/hooks.json
//  4. Global rules → ~/.codeium/windsurf/memories/global_rules.md
//
// NOTE: Script constants below must stay in sync with plugin/windsurf/scripts/.
package windsurf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/plugin"
)

// Installer installs mnemo into a Windsurf environment.
type Installer struct{}

// Install configures mnemo in the local Windsurf environment.
// If dryRun is true, prints what would change without writing anything.
func (i Installer) Install(dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	hooksDir := filepath.Join(home, ".local", "share", "mnemo", "windsurf-hooks")
	windsurfHooksJSON := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	windsurfMCPJSON := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")

	if dryRun {
		fmt.Println("mnemo setup --windsurf --dry-run (no changes will be written)")
	} else {
		fmt.Println("mnemo setup --windsurf")
	}
	fmt.Println("───────────────────────────────────────")

	if err := previewOrWriteHooks(hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	if err := previewOrRegisterMCP(windsurfMCPJSON, dryRun); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := previewOrInjectHooksJSON(windsurfHooksJSON, hooksDir, dryRun); err != nil {
		return fmt.Errorf("hooks.json: %w", err)
	}

	windsurfMemoriesDir := filepath.Join(home, ".codeium", "windsurf", "memories")
	if err := previewOrWriteGlobalRules(windsurfMemoriesDir, dryRun); err != nil {
		return fmt.Errorf("global_rules.md: %w", err)
	}

	if !dryRun {
		fmt.Println("\nDone. Restart Windsurf to activate mnemo.")
	}
	return nil
}

// Uninstall is not yet implemented.
func (i Installer) Uninstall() error {
	return fmt.Errorf("uninstall not yet implemented for Windsurf")
}

// ─── Global rules ─────────────────────────────────────────────────────────────

const globalRulesDoc = `## mnemo — Persistent Memory Protocol

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

### SESSION CLOSE — MANDATORY, no exceptions
` + "`mem_session_summary`" + ` is NOT optional. It is the final step of every session.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
No session ends without ` + "`mem_session_summary`" + `.
`

func previewOrWriteGlobalRules(dir string, dryRun bool) error {
	rulesPath := filepath.Join(dir, "global_rules.md")

	const marker = "## mnemo — Persistent Memory Protocol"

	if dryRun {
		data, err := os.ReadFile(rulesPath)
		if err == nil && strings.Contains(string(data), marker) {
			fmt.Println("\n[~/.codeium/windsurf/memories/global_rules.md] — already contains mnemo protocol, no change needed")
		} else {
			fmt.Printf("\n[~/.codeium/windsurf/memories/global_rules.md] — would append mnemo protocol\n")
		}
		return nil
	}

	data, err := os.ReadFile(rulesPath)
	if err == nil && strings.Contains(string(data), marker) {
		fmt.Println("✓ ~/.codeium/windsurf/memories/global_rules.md — already up to date")
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Append to existing content or create new file.
	content := string(data)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content += "\n"
	}
	if len(content) > 0 {
		content += "\n"
	}
	content += globalRulesDoc

	if err := os.WriteFile(rulesPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("✓ ~/.codeium/windsurf/memories/global_rules.md updated")
	return nil
}

// ─── Hook scripts ─────────────────────────────────────────────────────────────
// Keep in sync with plugin/windsurf/scripts/.

const preUserPromptScript = `#!/bin/bash
# mnemo — pre_user_prompt hook for Windsurf
# Fires before every prompt. First occurrence of a trajectory_id creates the
# session, emits general context, and searches for memories relevant to the
# opening prompt.
#
# Input: {
#   "agent_action_name": "pre_user_prompt",
#   "trajectory_id": "...", "execution_id": "...", "timestamp": "...",
#   "tool_info": { "user_prompt": "..." }
# }

INPUT=$(cat)
TRAJECTORY_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('trajectory_id',''))" 2>/dev/null)
PROMPT=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_info',{}).get('user_prompt',''))" 2>/dev/null)

[ -z "$TRAJECTORY_ID" ] && exit 0

# ─── Ensure ~/.codeium/windsurf/memories/global_rules.md has mnemo protocol ──
WINDSURF_MEMORIES_DIR="${HOME}/.codeium/windsurf/memories"
GLOBAL_RULES="${WINDSURF_MEMORIES_DIR}/global_rules.md"

if ! grep -q "## mnemo — Persistent Memory Protocol" "$GLOBAL_RULES" 2>/dev/null; then
  mkdir -p "$WINDSURF_MEMORIES_DIR"
  [ -s "$GLOBAL_RULES" ] && printf "\n\n" >> "$GLOBAL_RULES"
  cat >> "$GLOBAL_RULES" << 'RULESEOF'
## mnemo — Persistent Memory Protocol

You have access to mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).

### PROACTIVE SAVE — do NOT wait for user to ask
Call mem_save IMMEDIATELY after ANY of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented/updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

Self-check after EVERY task: did I just make a decision, fix a bug, learn something, or establish a convention? If yes, call mem_save NOW.

### SEARCH MEMORY when:
- User asks to recall anything
- Starting work on something that might have been done before
- User mentions a topic you have no context on

### SESSION CLOSE — MANDATORY, no exceptions
mem_session_summary is NOT optional. It is the final step of every session.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
No session ends without mem_session_summary.
RULESEOF
fi
# ─────────────────────────────────────────────────────────────────────────────

WORKSPACE="$(pwd)"
PROJECT=$(git -C "$WORKSPACE" rev-parse --show-toplevel 2>/dev/null | xargs basename 2>/dev/null)
[ -z "$PROJECT" ] && PROJECT=$(git -C "$WORKSPACE" remote get-url origin 2>/dev/null | sed 's/\.git$//' | sed 's|.*[/:]||')
[ -z "$PROJECT" ] && PROJECT=$(basename "$WORKSPACE")

# Only act on the first prompt of a conversation (session start)
IS_KNOWN=$(mnemo session exists "$TRAJECTORY_ID" 2>/dev/null)
[ "$IS_KNOWN" = "true" ] && exit 0

# New conversation — register session and emit general context
mnemo session start "$TRAJECTORY_ID" --project "$PROJECT" --dir "$WORKSPACE" 2>/dev/null || true
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

const postCascadeResponseScript = `#!/bin/bash
# mnemo — post_cascade_response_with_transcript hook for Windsurf
# Fires after a conversation response. Reads transcript JSONL for passive
# capture, then closes the mnemo session.
#
# Input: {
#   "agent_action_name": "post_cascade_response_with_transcript",
#   "trajectory_id": "...", "execution_id": "...", "timestamp": "...",
#   "tool_info": { "transcript_path": "/path/to/{trajectory_id}.jsonl" }
# }

INPUT=$(cat)
TRAJECTORY_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('trajectory_id',''))" 2>/dev/null)
TRANSCRIPT_PATH=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_info',{}).get('transcript_path',''))" 2>/dev/null)

[ -z "$TRAJECTORY_ID" ] && exit 0

WORKSPACE="$(pwd)"
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
    mnemo capture "$CONTENT" --session "$TRAJECTORY_ID" --project "$PROJECT" 2>/dev/null || true
  fi
fi

OBS_COUNT=$(mnemo session obs-count "$TRAJECTORY_ID" 2>/dev/null)
mnemo session end "$TRAJECTORY_ID" 2>/dev/null || true

if [ "${OBS_COUNT:-0}" = "0" ]; then
  printf "\n[mnemo] warning: session ended with 0 memories saved.\n" >&2
fi

exit 0
`

func previewOrWriteHooks(dir string, dryRun bool) error {
	scripts := map[string]string{
		"pre-user-prompt.sh":       preUserPromptScript,
		"post-cascade-response.sh": postCascadeResponseScript,
	}

	if dryRun {
		fmt.Printf("[~/.local/share/mnemo/windsurf-hooks/] — would write:\n")
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

// ─── MCP registration via ~/.codeium/windsurf/mcp_config.json ────────────────

func previewOrRegisterMCP(mcpPath string, dryRun bool) error {
	mnemoPath := plugin.ResolveBinaryPath("mnemo")

	if dryRun {
		fmt.Printf("\n[~/.codeium/windsurf/mcp_config.json] — would add:\n  mnemo: %s mcp --tools=agent\n", mnemoPath)
		return nil
	}

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
		fmt.Println("✓ MCP server mnemo — already registered in ~/.codeium/windsurf/mcp_config.json")
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
	fmt.Println("✓ MCP server mnemo registered in ~/.codeium/windsurf/mcp_config.json")
	return nil
}

// ─── ~/.codeium/windsurf/hooks.json ──────────────────────────────────────────

type windsurfHook struct {
	Command string `json:"command"`
}

type windsurfHooksConfig struct {
	Hooks map[string][]windsurfHook `json:"hooks"`
}

func previewOrInjectHooksJSON(path, hooksDir string, dryRun bool) error {
	entries := map[string]string{
		"pre_user_prompt":                      filepath.Join(hooksDir, "pre-user-prompt.sh"),
		"post_cascade_response_with_transcript": filepath.Join(hooksDir, "post-cascade-response.sh"),
	}

	if dryRun {
		fmt.Printf("\n[~/.codeium/windsurf/hooks.json] — would add hooks:\n")
		for event, script := range entries {
			fmt.Printf("  %s: %s\n", event, script)
		}
		return nil
	}

	var config windsurfHooksConfig
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}
	if config.Hooks == nil {
		config.Hooks = make(map[string][]windsurfHook)
	}

	added := 0
	for event, script := range entries {
		if hookExists(config.Hooks[event], script) {
			continue
		}
		config.Hooks[event] = append(config.Hooks[event], windsurfHook{Command: script})
		added++
	}

	if added == 0 {
		fmt.Println("✓ ~/.codeium/windsurf/hooks.json — already up to date")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ ~/.codeium/windsurf/hooks.json updated (%d hooks added)\n", added)
	return nil
}

func hookExists(hooks []windsurfHook, command string) bool {
	for _, h := range hooks {
		if strings.EqualFold(h.Command, command) {
			return true
		}
	}
	return false
}
