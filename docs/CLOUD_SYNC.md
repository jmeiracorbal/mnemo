# Cloud sync

mnemo cloud sync is local-first and complete-by-default:

- the SQLite store remains the local operational copy;
- every local session, observation, prompt, update, and logical delete is represented as an idempotent sync mutation;
- the cloud journal is append/upsert-only from each client (`origin_id`, `client_seq`), so retrying the same batch does not duplicate remote rows;
- memory deletion is logical and synced as a delete mutation so decisions are not lost.

## Setup

Configure credentials interactively:

```bash
mnemo setup cloud
```

Credentials are saved to `~/.config/mnemo/cloud.toml` following the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/). The file format is:

```toml
[cloud]
provider   = "turso"
url        = "libsql://<your-db>.turso.io"
key        = "<auth-token>"
client_id  = "<stable-device-or-agent-id>"
```

Environment variables override the file when set (useful for CI or Docker):

| Variable | Description |
|---|---|
| `MNEMO_CLOUD_URL` | Turso/libSQL database URL (`libsql://` or `https://`) |
| `MNEMO_CLOUD_KEY` | Auth token |
| `MNEMO_CLOUD_CLIENT_ID` | Stable identifier for this client |
| `MNEMO_CLOUD_TARGET` | Sync target key (optional, defaults to `cloud`) |

Priority order: CLI flags → environment variables → `cloud.toml` → defaults.

### Setup subcommands

```bash
mnemo setup cloud                          # interactive setup
mnemo setup cloud --validate               # test credentials without saving
mnemo setup cloud --delete                 # remove saved credentials
mnemo setup cloud --non-interactive \
  --url=libsql://... --key=... --client-id=...   # scripted setup
```

## Commands

```bash
mnemo sync run          # push then pull (backfills missing queue entries before push)
mnemo sync push         # backfill all local data and upload pending mutations
mnemo sync pull         # apply remote mutations locally
mnemo sync status       # read-only local state; no cloud contact or queue backfill
```

All write commands are idempotent. `sync run` skips rows whose `origin_id` equals this client's `client_id` while still advancing the local pull cursor. The pull cursor is a remote high-water mark, so gaps in visible cloud sequence numbers are valid when filtered rows exist.

## Queue recovery and dependency ordering

The canonical local tables are the source of truth. Before a push, mnemo
reconciles the local queue with those rows and rebuilds missing or stale queue
entries, including soft-deleted rows. Each mutation contains one row from one
canonical table; related rows are not nested into aggregate payloads.

Pending mutations are sent in foreign-key dependency order: projects first,
then reference metadata and provenance, sessions, observations and prompts,
and finally tags and reviews. This ordering is applied by every client so a
fresh or reset cloud database can be rebuilt without creating orphaned
references. The queue and other local synchronization metadata can be lost and
reconstructed without losing canonical data.

Opening the store, including MCP startup, does not perform this full queue
reconciliation. Push operations (`sync push`, `sync run`, or MCP sync modes
`push`/`run`) perform it when synchronization state needs to be rebuilt.
The CLI and MCP status operations only read the local state and pending queue,
keeping agent handshakes and status checks independent from queue rebuilding.

Flags available on `run`, `push`, and `pull`:

```
--url          Override cloud URL for this run
--key          Override auth token for this run
--client-id    Override client ID for this run
--target       Sync target key (default: cloud)
--batch        Mutation batch size (default: 25)
--json         Output result as JSON
```

## Provider

The current provider is **Turso** (libSQL/SQLite). The cloud database mirrors the local schema exactly — `sessions`, `observations`, `user_prompts`, FTS tables such as `user_prompts_fts`, `sync_mutations` and provenance tables. Migrations are applied automatically on first sync.

The `CloudProvider` interface (`internal/cloudsync/provider.go`) is designed for future providers (e.g. a hosted mnemo-cloud service).

## Diagnostics

`mnemo doctor` reports the cloud connection status as a `cloud` check:

```
✓ ok       cloud sync: connected (turso @ libsql://your-db.turso.io)
```

If credentials are missing: `cloud sync: not configured (optional — run 'mnemo setup cloud' to enable)`.

If the connection fails: `! warning  cloud sync: connection failed: <reason>`.

The doctor ping uses a 5-second timeout so it never hangs the diagnostic run.
