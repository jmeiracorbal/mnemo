# Contributing to mnemo

Thanks for helping improve mnemo. This project ships a Go binary and agent
integrations (hooks, MCP, skills). Keep changes small, verifiable, and aligned
with the core invariants in [`AGENTS.md`](AGENTS.md).

## Before you start

1. Open an issue for non-trivial changes when possible.
2. Read the maintainer guide for your change area:

| Change area | Read first |
|---|---|
| Hooks, plugin setup, agent integration | [`docs/maintainers/agent-integrations.md`](docs/maintainers/agent-integrations.md) |
| Versions, installer, releases | [`docs/maintainers/releases.md`](docs/maintainers/releases.md) |
| Database schema or sqlc | [`docs/maintainers/migrations.md`](docs/maintainers/migrations.md) |

## Development setup

Requirements: Go (see `go.mod`), macOS or Linux.

```bash
git clone https://github.com/jmeiracorbal/mnemo.git
cd mnemo
go build -o ~/.local/bin/mnemo ./cmd/mnemo/
```

For local agent wiring while developing:

```bash
mnemo setup refresh --agent=all
mnemo doctor --agent=all --path=.
```

## Verification

Every code change must pass:

```bash
go test ./...
go build ./...
git diff --check
```

For hook, plugin, or setup changes, also follow the validation checklist in the
relevant maintainer guide.

## Pull requests

- Keep the diff focused. Prefer one concern per PR.
- Explain **why**, not only what changed.
- Call out user impact, the minimal fix, and verification evidence.
- Distinguish bugs, documentation updates, and design decisions.
- Do not edit existing database migrations. Schema changes need a new
  incremental migration and an updated `database/target_schema.sql`.
- Do not invent project IDs from paths. Project identity is only the `id` in
  `.mnemo`.

## Code of conduct

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

Do not open public issues for vulnerabilities. See [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
