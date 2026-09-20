<p align="center">
  <img src="assets/brand/mnemo-logo.png" alt="mnemo logo" width="128" height="128">
</p>

<h1 align="center">mnemo</h1>

<p align="center">
  <strong>Persistent memory for AI coding agents.</strong>
</p>

<p align="center">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://go.dev"><img alt="Go" src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo/releases"><img alt="Status" src="https://img.shields.io/badge/status-alpha-orange"></a>
  <a href="https://sqlite.org"><img alt="Storage" src="https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="Platform" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache%202.0-blue"></a>
</p>

<p align="center">
  <img alt="Claude Code" src="https://img.shields.io/badge/Claude%20Code-supported-6B46C1?logo=claudecode&logoColor=white">
  <img alt="Codex" src="https://img.shields.io/badge/Codex-supported-00A67E?logo=openai&logoColor=white">
  <img alt="Cursor" src="https://img.shields.io/badge/Cursor-supported-111111?logo=cursor&logoColor=white">
  <img alt="OpenCode" src="https://img.shields.io/badge/OpenCode-supported-F97316?logo=opencode&logoColor=white">
  <img alt="Pi" src="https://img.shields.io/badge/Pi-supported-0EA5E9">
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="#why-mnemo">Why mnemo?</a> ·
  <a href="#supported-agents">Agents</a> ·
  <a href="#documentation">Docs</a> ·
  <a href="ROADMAP.md">Roadmap</a>
</p>

---

## What is mnemo?

mnemo is a local memory layer for agentic development. It stores decisions, bugs, conventions, discoveries and session summaries in SQLite, then exposes them to agents through MCP or native tools, hooks and portable Agent Skills. A local event controller is the sole normal SQLite writer: agents publish durable lifecycle events and memory commands instead of opening the database.

Instead of spreading project knowledge across `MEMORY.md`, native editor memory, chat transcripts and human notes, mnemo gives every supported agent the same project-scoped source of truth.

## Quick Start

Install the binary and configure your detected agents:

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | MNEMO_VERSION=v1.0.0-alpha.4 bash
```

This pins the current alpha. Unpinned installs and `mnemo update` follow stable releases by default; use `mnemo update --prerelease` to opt into prereleases.

Activate mnemo in a project:

```bash
cd your-project
mnemo init --agent=all
```

Check that everything is wired correctly:

```bash
mnemo doctor --agent=all --path=.
```

In your agent, use `mem_save` to record a decision and `mem_search` to find it later. The CLI can also search existing memories:

```bash
mnemo search "SQLite" --project "$(mnemo json id < .mnemo)"
```

The controller is installed as a per-user service during setup. See [Durable events](https://github.com/jmeiracorbal/mnemo/wiki/Durable-Events) for delivery and failure semantics.

## Why mnemo?

| Problem | mnemo gives you |
|---|---|
| Agents forget decisions between sessions | Durable project memory in `~/.mnemo/memory.db` |
| Hooks or plugins can lose writes or race each other | One local event controller with durable delivery and idempotent SQLite transactions |
| Markdown memory files drift or conflict | Structured observations, tags, topic keys and review states |
| Global hooks can be risky | Project opt-in via a `.mnemo` marker; projects without it are ignored |
| Setup breaks silently | `mnemo doctor` and `mnemo setup status` explain exactly what is configured |
| Duplicated projects/memories accumulate | Project merge tools and memory curation workflows |

## Features

| Feature | What it does |
|---|---|
| **Project-scoped activation** | Global hooks only run when a project contains a valid `.mnemo` marker. |
| **MCP tools** | Agents can call `mem_save`, `mem_search`, `mem_context`, `mem_current_project`, `mem_doctor` and more. |
| **Durable event controller** | A per-user JetStream service is the single normal SQLite writer; publishers never open the store. |
| **Controller-owned sessions** | Agent-native execution IDs bind events and memory tools to one canonical session. |
| **Portable Agent Skills** | Skills teach compatible agents when and how to use mnemo without falling back to native memory. |
| **Passive capture** | Memory tools extract useful learnings from agent-provided output. |
| **Agent provenance** | Records SQL-queryable agent, source, tool, model and MCP client metadata for writes that provide it. |
| **Diagnostics** | `mnemo doctor` checks project activation, global setup, MCP, hooks, competing memory surfaces and database migration health. |
| **Database safety** | Safe schema migrations run automatically; `mnemo db migrate --check` validates the local store for CI or troubleshooting. |
| **Self-update** | Released binaries check for newer releases on interactive CLI use and can confirm, download and install with `mnemo update`. |
| **Programmable CLI** | Cobra-generated help keeps the command menu and nested subcommands aligned with the executable. |
| **Project maintenance** | `mnemo projects list`, `mnemo projects merge` and `mnemo projects rename` help curate duplicate or unclear project identities. |
| **Memory curation** | `mnemo memories review` surfaces duplicate or conflicting observations for approved repair. |

## Associated modules

mnemo uses these published modules as the single source of truth for its shared
integration contracts:

| Module | Purpose |
|---|---|
| [`mnemo-adapters`](https://github.com/jmeiracorbal/mnemo-adapters) | Agent adapter interfaces, registrations and mappings used by the CLI, controller and MCP integration. |
| [`mnemo-events`](https://github.com/jmeiracorbal/mnemo-events) | Durable event and command contracts shared by publishers, the controller and store. |

## Supported Agents

| Agent | MCP | Hooks / runtime | Global instructions | Skill access | Status |
|---|---:|---:|---:|---:|---|
| Claude Code | Yes | Plugin hooks or installer setup | Yes | Yes | Supported |
| Codex | Yes | Session hooks | Yes | Yes | Supported |
| Cursor | Yes | Prompt hook | Yes | Yes | Supported |
| OpenCode | Yes | Plugin events | Yes | Yes | Supported |
| Pi | Native tools | Native extension | Yes | Yes | Supported |

Global setup is installed once. Project activation stays local and opt-in:

```text
project/
├── .mnemo      # project ID + activated agents, ignored by git
├── AGENTS.md   # shared project memory authority
├── CLAUDE.md   # Claude-specific rules when selected
├── .cursor/    # Cursor rules when selected
└── .pi/        # Pi prompt extensions when selected
```

## See it in action

```text
$ mnemo doctor --agent=all --path=.
status: ok
checks: project marker, binary, MCP, hooks, instructions, store

$ mnemo context myapp
## Memory from Previous Sessions
- Chose SQLite FTS5 for local search.
- Refresh hooks must keep executable permissions.

$ mnemo memories review --project=myapp
No potential memory conflicts found.
```

## Installation options

| Path | Use when | Command |
|---|---|---|
| Current alpha | You want to try `v1.0.0-alpha.4` | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; MNEMO_VERSION=v1.0.0-alpha.4 bash</code> |
| Latest stable | You want the latest stable binary plus detected agent setup | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; bash</code> |
| Explicit agent | You only want one integration | `bash -s -- --agent=codex` |
| All agents | You want every supported integration prepared | `bash -s -- --agent=all` |
| Claude plugin | You use Claude Code's plugin marketplace | `claude plugin install mnemo@mnemo` |
| Source build | You develop mnemo itself | `go build -o ~/.local/bin/mnemo ./cmd/mnemo/` |

Read the complete [installation guide](https://github.com/jmeiracorbal/mnemo/wiki/Installation).

### Updates

Released mnemo binaries check GitHub Releases during interactive CLI use. When
a newer release exists, mnemo itself prints the installed/latest versions and
asks before changing anything:

```bash
mnemo update
mnemo update --prerelease --check
mnemo update --prerelease
mnemo update --yes --agent=all
mnemo update --check --json
```

By default, `mnemo update` checks stable releases. `--prerelease` checks all
published releases and selects the newest version, including alphas and betas.
`mnemo update` downloads the official installer, pins it to the detected latest
release and refreshes mnemo's agent integration files after installing. It does
not update Claude Code, Codex, Cursor or other agent applications themselves.
Restart active agent sessions after updating so they reload the refreshed
binary, hooks and skills. Update checks are skipped in MCP, hook and JSON-output
paths so integrations remain machine-readable.

### Codex hook review

Codex protects every hook in `~/.codex/hooks.json` with an interactive trust
review. mnemo currently installs Codex `SessionStart` and `Stop` hooks; the same
Codex trust mechanism will also apply whenever a mnemo-owned hook is added or
its command changes.

If Codex says a mnemo hook needs review, or if `mnemo setup status --agent=codex`
shows the Codex `Hooks` column as `review`, open Codex normally and approve the
interactive hook prompt (press `a` or follow the prompt shown by Codex). Codex
will then write the matching `trusted_hash` entries under `[hooks.state]` in
`~/.codex/config.toml`. Re-run:

```bash
mnemo setup status --agent=codex
mnemo doctor --agent=codex --path=.
```

to confirm the Codex hooks are trusted and active. Do not manually copy hashes
between machines; approve hooks in the Codex UI so the hash matches the local
hook command.

## Documentation

| Guide | Contents |
|---|---|
| [Wiki home](https://github.com/jmeiracorbal/mnemo/wiki) | User documentation and navigation. |
| [Installation](https://github.com/jmeiracorbal/mnemo/wiki/Installation) | Binary, controller service, project activation and verification. |
| [Agent integrations](https://github.com/jmeiracorbal/mnemo/wiki/Agent-Integrations) | Native IDs, hooks, tools and project marker. |
| [Durable events](https://github.com/jmeiracorbal/mnemo/wiki/Durable-Events) | Controller architecture, delivery and failure behavior. |
| [CLI reference](https://github.com/jmeiracorbal/mnemo/wiki/CLI-Reference) | Active commands, MCP tools and search modes. |
| [Troubleshooting](https://github.com/jmeiracorbal/mnemo/wiki/Troubleshooting) | Diagnostics and controller recovery. |
| [Storage](https://github.com/jmeiracorbal/mnemo/wiki/Storage-and-Migrations) | SQLite, migrations and sqlc workflow. |
| [Roadmap](ROADMAP.md) | Planned product and maintenance work. |

## Design principles

- **Local-first:** memory stays on your machine in SQLite.
- **Agent-neutral:** one memory authority across supported coding agents.
- **Opt-in by project:** global integrations are inert without `.mnemo`.
- **Diagnosable:** every setup surface can be checked without mutation.
- **Repairable:** duplicate project identities and memory conflicts are visible and fixable through CLI primitives.

## License

[Apache 2.0](LICENSE): you may use, modify, and distribute freely, but must retain the copyright notice and include the [NOTICE](NOTICE) file in all distributions.
