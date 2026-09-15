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

The event includes either an existing `execution_key` or the adapter's
`agent` + `native_id`. Only the Go controller derives the canonical execution
key. TypeScript hooks and extensions must not calculate identifiers or use a
path, PID, latest session, temporary file or direct SQLite fallback.

The controller acknowledges an event only after one SQLite transaction records
its idempotency key and applies its effect. A failed or unbound event remains in
JetStream for retry; no event is silently downgraded to a direct write.

## MCP boundary

`mnemo mcp` does not open the SQLite store. Read operations are forwarded to
the controller over local NATS request/reply. Mutating operations are published
to the durable `MNEMO_COMMANDS` JetStream stream and the controller replies
only after it has applied the operation. This keeps the MCP process, agent
hooks and extensions on the same local controller boundary.

## Event types

- `execution.started` — creates and binds the controller-owned session.
- `execution.closed` — closes the bound session.
- `session.compacted` — records compaction for the bound session.
- `workspace.file_changed` — creates a `file_change` observation.
- `git.commit_created` — creates a `decision` observation.

## Adapter rule

Every supported agent has an adapter under `adapters/`. An agent can publish
only events for which its adapter exposes a native execution identity. Missing
adapter capability is a configuration error, not a reason to add an
agent-specific fallback.
