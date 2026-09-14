# Durable hook events

mnemo can carry typed agent-hook events through a local, embedded NATS
JetStream controller. This is an opt-in transport boundary: hook publishers do
not open SQLite, and SQLite writes happen only in the controller after a
durable event is consumed.

## Project configuration

Create the project-root `config.toml` with an explicit local port:

```toml
[mnemo.events]
port = 4222
```

There is deliberately no implicit shared port. Two project controllers cannot
silently bind the same address, and a missing or invalid configuration fails
before a hook can publish. The listener is always `127.0.0.1`; it is not a
remote API.

JetStream data is held under `~/.mnemo/events/<project-id>/`. This is required
for durable delivery: no-loss delivery cannot be achieved with an in-memory
queue. The user does not need to install Redis or NATS separately; the server
is embedded in the `mnemo` binary.

## Lifecycle

Start one controller for the active project before starting its agent:

```bash
mnemo events serve --project "$PROJECT_ID" --directory "$PWD"
```

The agent-launch adapter generates one opaque `MNEMO_EXECUTION_KEY` for its
execution and passes it to both the MCP subprocess and supported hooks. MCP
binds that key to its private session ID. The hook event contains the key, not
the private session ID.

The controller only acknowledges an event after the same SQLite transaction
both records its idempotency key and applies its effect. If the session binding
does not exist yet or SQLite fails, JetStream retains the event for retry.

## Event contract

The supported initial event types are:

- `session.compacted` — updates the bound session compaction timestamp.
- `workspace.file_changed` — payload requires `path` and `action`; creates a
  `file_change` observation.
- `git.commit_created` — payload requires `hash` and `message`; creates a
  `decision` observation.

For diagnostics, a publisher can be invoked directly:

```bash
mnemo events publish \
  --project "$PROJECT_ID" \
  --directory "$PWD" \
  --execution-key "$MNEMO_EXECUTION_KEY" \
  --type workspace.file_changed \
  --payload '{"path":"README.md","action":"modified"}'
```

An event whose controller is unavailable fails explicitly. It is never routed
to direct SQLite, a temporary file, a PID lookup, a path-derived session, or a
"latest session" fallback.

## Adapter rollout

`adapters/` defines a concrete identity adapter for every supported agent. The
transport is ready for launch adapters to use, but existing shipped hooks stay
context-only until each agent can receive the same execution key in its hook
and MCP subprocess. This avoids enabling partial delivery semantics for only
some agents.
