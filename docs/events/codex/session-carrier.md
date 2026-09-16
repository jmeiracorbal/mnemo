# Codex MCP session carrier

## Status

This is an empirical compatibility record, not an MCP-standard contract.
Revalidate it after upgrading Codex or when the Codex adapter test fails.

## Capture

- **Date:** 2026-09-16
- **Client:** Codex CLI 0.154.0
- **Method:** local probe MCP recording inbound `tools/call` requests.
- **Result:** Codex supplied the following metadata on every observed
  `tools/call`. Its `session_id` exactly matched the value sent to the Codex
  `SessionStart` hook on stdin.

```json
{
  "params": {
    "_meta": {
      "x-codex-turn-metadata": {
        "session_id": "01a0a923-11c1-7e63-b730-c09ec0dd341a",
        "thread_id": "01a0a923-11c1-7e63-b730-c09ec0dd341a",
        "turn_id": "01a0a923-4987-7ce2-84d5-5b181b2db5d8"
      },
      "threadId": "01a0a923-11c1-7e63-b730-c09ec0dd341a"
    }
  }
}
```

## Adapter contract

mnemo uses only `params._meta.x-codex-turn-metadata.session_id` as the native
execution identity. `thread_id`, `turn_id`, and `threadId` are observed context
only; they are not identity fallbacks. Missing or malformed `session_id` is an
input error and must not be replaced with a PID, path, MCP instance ID, or a
recent session.
