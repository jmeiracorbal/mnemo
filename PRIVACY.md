# Privacy Policy

**Last updated:** 2026-09-17

## Summary

mnemo does not include telemetry or a hosted memory service. Memory and event data stay on your machine. Installation and update checks may contact GitHub.

## Data storage

Memories, sessions, and observations are stored in `~/.mnemo/memory.db`. The local controller also stores durable event data in `~/.mnemo/events/` and its configuration in `~/.mnemo/config.toml`. It binds to `127.0.0.1` and does not expose a remote memory API.

## No data collection

mnemo does not include telemetry or analytics and does not send memory contents to a hosted mnemo service. Requests to GitHub for installation or updates may disclose ordinary connection metadata, such as your IP address, to GitHub.

The installer downloads releases from GitHub. Interactive CLI use may check GitHub Releases for updates; `mnemo update` downloads the installer after confirmation. Agent tools use local MCP or native transports and communicate with the controller on localhost.

## What mnemo stores locally

mnemo writes memory content saved by you or an agent, as well as lifecycle and observation events published by supported integrations:

- Memory entries (titles, content, types) you create during coding sessions
- Session metadata (session IDs, project names, timestamps)
- Observations captured from agent-provided output or supported local event sources
- Durable event and command records used for controller delivery and idempotency

All of this data stays on your machine and is fully under your control.

## Deleting your data

First run `mnemo setup uninstall --agent=all` to remove managed integrations and stop the controller service. After backing up anything you want to keep, remove `~/.mnemo/` to delete the database, event log and configuration. The installer location of the binary may differ; check your PATH or `MNEMO_INSTALL_DIR` before removing it.

## Open source

mnemo is open source under the [Apache 2.0 License](LICENSE). You can inspect the full source code at [github.com/jmeiracorbal/mnemo](https://github.com/jmeiracorbal/mnemo).

## Contact

For questions, open an issue at [github.com/jmeiracorbal/mnemo/issues](https://github.com/jmeiracorbal/mnemo/issues).
