# Cursor MCP session carrier

## Status

This is an empirical compatibility record, not an MCP-standard contract.
Revalidate it after upgrading Cursor or when the Cursor adapter test fails.

## Capture

- **Date:** 2026-09-16
- **Client:** Cursor 1.0.0 (MCP protocol version `2025-11-25`)
- **Method:** local probe MCP recording the inbound `initialize` request and
  `tools/call` requests from the MCP child process environment.
- **Result:** Cursor does not supply a session or conversation identity in the
  MCP `initialize` handshake, in the MCP child process environment, or in any
  `tools/call` request metadata.

```json
{
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "capabilities": { "elicitation": { "form": {} } },
    "clientInfo": { "name": "Cursor", "version": "1.0.0" }
  }
}
```

No `_meta`, no `conversation_id`, no `session_id`. MCP child process environment
contained only `MNEMO_PROBE_*` variables — nothing Cursor-session-specific.

`tools/call` requests equally carried no `_meta` session field.

## Side-channel carrier

Cursor does supply `conversation_id` in the `beforeSubmitPrompt` hook's stdin
JSON payload. This hook fires before every prompt submission, which guarantees
the value is written before any MCP tool call for that turn arrives.

```json
{
  "conversation_id": "...",
  "generation_id": "...",
  "prompt": "...",
  "workspace_roots": ["..."],
  "transcript_path": null,
  "hook_event_name": "beforeSubmitPrompt"
}
```

`session_id` (carried by `sessionStart` / `sessionEnd` hook events) is
documented by Cursor as equivalent to `conversation_id`.

## Adapter contract

mnemo reads `conversation_id` from a filesystem side-channel written by the
`beforeSubmitPrompt` hook. The canonical path is:

```
${TMPDIR:-/tmp}/mnemo-cursor-<project-id>
```

where `<project-id>` is the UUID from the project's `.mnemo` file.

The hook writes the value with `printf '%s'` before the prompt is submitted; the
MCP runtime reads it on each memory-writing tool call via `SideChannelAdapterFor`
and passes it to the controller as `agent + native_id`. The controller derives
the execution key deterministically — the MCP process never computes or submits
it directly.

A missing or empty side-channel file is an input error and must not be replaced
with a PID, path, MCP instance ID, or recent session.

## Session lifecycle

Session open and close are both owned by the MCP stdio connection, not by hooks:

- **Open:** first memory-writing tool call triggers `BindExecutionSession`,
  linking the `conversation_id`-derived execution key to the MCP instance session.
- **Close:** Cursor closing the MCP stdio connection triggers
  `CloseMCPInstanceSessions`.

The Cursor `stop` hook is intentionally a no-op for session lifecycle.
