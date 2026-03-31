# Privacy Policy

**Last updated:** 2026-03-29

## Summary

mnemo does not collect, transmit, or store any data outside your local machine.

## Data storage

All data saved by mnemo — memories, sessions, and observations — is stored exclusively in a local SQLite database at `~/.mnemo/memory.db` on your machine. This file is created automatically on first run and never leaves your device.

## No data collection

mnemo does not:

- Collect or transmit any personal data
- Send data to external servers
- Include telemetry or analytics of any kind
- Make network requests (the MCP server runs locally via stdio)

## What mnemo stores locally

The only data mnemo writes is what you or your AI agent explicitly saves:

- Memory entries (titles, content, types) you create during coding sessions
- Session metadata (session IDs, project names, timestamps)
- Observations captured passively from session transcripts on your machine

All of this data stays on your machine and is fully under your control.

## Deleting your data

To remove all mnemo data, delete `~/.mnemo/memory.db`. To remove the binary, delete `~/.local/bin/mnemo`. Hook scripts are in `~/.claude/hooks/`, `~/.cursor/hooks/`, and `~/.codeium/windsurf/hooks/`.

## Open source

mnemo is open source under the [Apache 2.0 License](LICENSE). You can inspect the full source code at [github.com/jmeiracorbal/mnemo](https://github.com/jmeiracorbal/mnemo).

## Contact

For questions, open an issue at [github.com/jmeiracorbal/mnemo/issues](https://github.com/jmeiracorbal/mnemo/issues).
