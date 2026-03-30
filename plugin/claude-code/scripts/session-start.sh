#!/bin/bash
# mnemo — SessionStart hook for Claude Code plugin
# Claude Code passes hook input via stdin as JSON:
#   { "session_id": "...", "cwd": "..." }

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null)
CWD=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cwd',''))" 2>/dev/null)

[ -z "$SESSION_ID" ] && exit 0
[ -z "$CWD" ] && CWD="$(pwd)"

# ─── Ensure ~/.claude/mnemo.md and CLAUDE.md are configured ──────────────────
CLAUDE_HOME="${HOME}/.claude"
MNEMO_MD="${CLAUDE_HOME}/mnemo.md"
CLAUDE_MD="${CLAUDE_HOME}/CLAUDE.md"

if [ ! -f "$MNEMO_MD" ]; then
  cat > "$MNEMO_MD" << 'MNEMOMD'
## mnemo — Persistent Memory Protocol

You have access to mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).

### MEMORY SYSTEM — mnemo is the ONLY memory system
**NEVER use the file-based memory system** (the one that writes `.md` files to `~/.claude/projects/*/memory/` and maintains a `MEMORY.md` index). That system is DISABLED for this workspace.
When asked to "save to memory", "remember this", or "guardar en memoria" — ALWAYS use `mem_save`. Never write files.

### PROACTIVE SAVE — do NOT wait for user to ask
Call `mem_save` IMMEDIATELY after ANY of these:
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

```
## Key Learnings
- <learning 1>
- <learning 2>
```

This enables mnemo to automatically extract and persist what you discovered.
Omit the section only if the task produced no learnings worth retaining.

### SESSION CLOSE — MANDATORY, no exceptions
`mem_session_summary` is NOT optional. It is the final step of every session, like a `defer` — it always runs.
Call it before ANY response that signals completion ("done", "listo", "ready", "finished", "completed").
Fields: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

If nothing was accomplished: call it anyway with Goal and Next Steps.
If the user says goodbye: call it before responding.
No session ends without `mem_session_summary`.
MNEMOMD
fi

if ! grep -q "@mnemo.md" "$CLAUDE_MD" 2>/dev/null; then
  printf "@mnemo.md\n" >> "$CLAUDE_MD"
fi
# ─────────────────────────────────────────────────────────────────────────────

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
