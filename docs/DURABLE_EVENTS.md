# Durable event controller

mnemo uses one installation-wide, local NATS JetStream controller for durable
agent events. Publishers never open SQLite. The controller is the only process
that consumes events and applies their SQLite transactions.

## Configuration and lifecycle

`mnemo setup refresh` creates the global configuration on first run:

```toml
# ~/.mnemo/config.toml
[events]
port = 4222
```

The port is a technical infrastructure setting; users can change it in that
file. The controller always binds to `127.0.0.1` and stores JetStream data in
`~/.mnemo/events/`. It is not a remote API and does not require Redis or a
separate NATS installation.

The same setup command registers the singleton controller with the native
per-user service manager:

| System | Registration |
| --- | --- |
| macOS | launchd `com.jmeiracorbal.mnemo.controller` LaunchAgent |
| Linux | systemd user unit `mnemo-controller.service` |
| Windows | Task Scheduler task `MnemoController` |

Each service starts `mnemo controller serve` at login and restarts it after a
failure. An MCP process only verifies that this controller is healthy; it never
starts another controller. If the service is unavailable, MCP startup and event
publication fail explicitly. Run `mnemo setup refresh --agent=all` after an
upgrade to install or refresh the service, and `mnemo setup uninstall` to stop
and remove it along with the selected agent setup.

## Publisher contract

Hook and extension publishers use the non-writing command below. It reads only
the global controller configuration and publishes to JetStream; it does not
open, migrate or write SQLite.

```bash
mnemo events publish \
  --project "$PROJECT_ID" \
  --directory "$PWD" \
  --agent pi \
  --native-id "$NATIVE_EXECUTION_ID" \
  --type execution.started \
  --payload "{\"directory\":\"$PWD\"}"
```

The event includes the adapter's `agent` + `native_id`. Only the Go controller
derives the canonical execution key. TypeScript hooks and extensions must not
calculate identifiers or use a path, PID, latest session, temporary file or
direct SQLite fallback.

The controller acknowledges an event only after one SQLite transaction records
its idempotency key and applies its effect. A failed or unbound event remains in
JetStream for retry; no event is silently downgraded to a direct write.

## MCP boundary

`mnemo mcp` does not open the SQLite store. Read operations are forwarded to
the controller over local NATS request/reply. Mutating operations are published
to the durable `MNEMO_COMMANDS` JetStream stream and the controller replies
only after it has atomically applied the operation and persisted its result.
On redelivery, the controller returns that stored result without repeating the
mutation. This keeps the MCP process, agent
hooks and extensions on the same local controller boundary.

When an agent supplies its native execution ID in MCP request metadata or the
inherited MCP process environment, mnemo binds the MCP-owned session through
the same controller command. For Codex, the adapter requires
`params._meta.x-codex-turn-metadata.session_id` on every `tools/call`; for
Claude Code, it requires `CLAUDE_CODE_SESSION_ID` in the MCP process. Each is
the same identifier its SessionStart hook publishes. A missing or malformed
carrier is an error, never a fallback to the MCP process identity. The
empirical records and revalidation rules live at
[events/codex/session-carrier.md](events/codex/session-carrier.md) and
[events/claudecode/session-carrier.md](events/claudecode/session-carrier.md).

Pi is deliberately different: its native extension reads
`ctx.sessionManager.getSessionId()` for every tool invocation and calls
`mnemo events invoke`. That command is a durable controller command, not an
MCP process and not a SQLite client. The controller derives the Pi execution
key in Go and creates or resolves its deterministic execution session in the
same transaction. Pi's static `mcp.json` entry is removed during setup because
it cannot carry this per-session identity.

## Event types

- `execution.started` — creates and binds the controller-owned session.
- `execution.closed` — closes the bound session.
- `session.compacted` — records compaction for the bound session.
- `agent.tool_result` — records a tool-use observation for the bound session.
- `workspace.file_changed` — creates a `file_change` observation.
- `git.commit_created` — creates a `decision` observation.

## Adapter rule

Every supported agent has an adapter under `adapters/`. An agent can publish
only events for which its adapter exposes a native execution identity. Missing
adapter capability is a configuration error, not a reason to add an
agent-specific fallback.
