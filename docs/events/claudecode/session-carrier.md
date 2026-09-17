# Claude Code MCP session carrier

## Status

This is an empirical compatibility record, not an MCP-standard contract.
Revalidate it after upgrading Claude Code or when the Claude Code adapter test
fails.

## Capture

- **Date:** 2026-09-15
- **Client:** Claude Code 2.1.272
- **Method:** local probe MCP inspecting the MCP child process environment and
  correlating it with the JSON payload received by the `SessionStart` hook.
- **Result:** Claude Code inherited `CLAUDE_CODE_SESSION_ID` into the MCP child
  process. Its value exactly matched the hook payload's `session_id`.

## Adapter contract

mnemo uses only `CLAUDE_CODE_SESSION_ID` as the Claude Code MCP native
execution identity. It is read by the MCP runtime for each memory-writing tool
call and sent to the controller as `agent + native_id`; the controller derives
the execution key. A missing or blank variable is an input error and must not
be replaced with a PID, path, MCP instance ID, or recent session.
