<p align="center">
  <img src="assets/brand/mnemo-logo.png" alt="mnemo logo" width="128" height="128">
</p>

<h1 align="center">mnemo</h1>

<p align="center">
  <strong>Persistent memory for AI coding agents.</strong>
</p>

<p align="center">
  Give Claude Code, Codex, Cursor, Windsurf, OpenCode, fx and Pi one shared local memory that survives sessions, compactions and agent switches.
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://go.dev"><img alt="Go" src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="Status" src="https://img.shields.io/badge/status-stable-brightgreen"></a>
  <a href="https://sqlite.org"><img alt="Storage" src="https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="Platform" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache%202.0-blue"></a>
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

mnemo is a local memory layer for agentic development. It stores decisions, bugs, conventions, discoveries and session summaries in SQLite, then exposes them back to agents through MCP tools, hooks and portable Agent Skills.

Instead of spreading project knowledge across `MEMORY.md`, native editor memory, chat transcripts and human notes, mnemo gives every supported agent the same project-scoped source of truth.

> [!IMPORTANT]
> mnemo does not automatically support every harness; it provides a stable memory contract that any harness can implement and validate.

## Quick Start

Install the binary and configure your detected agents:

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
```

Activate mnemo in a project:

```bash
cd your-project
mnemo init --agent=all
```

Check that everything is wired correctly:

```bash
mnemo doctor --agent=all --path=.
```

Save and search memory manually from the CLI:

```bash
mnemo save "Use SQLite FTS5" "Search stays local, fast and dependency-light." --type decision --project myapp
mnemo search "SQLite" --project myapp
```

## Why mnemo?

| Problem | mnemo gives you |
|---|---|
| Agents forget decisions between sessions | Durable project memory in `~/.mnemo/memory.db` |
| Different agents keep different memories | One shared layer for Claude Code, Codex, Cursor, Windsurf, OpenCode, fx and Pi |
| Markdown memory files drift or conflict | Structured observations, tags, topic keys and review states |
| Global hooks can be risky | Project opt-in via a `.mnemo` marker; projects without it are ignored |
| Setup breaks silently | `mnemo doctor` and `mnemo setup status` explain exactly what is configured |
| Duplicated projects/memories accumulate | Project merge tools and memory curation workflows |

## Features

| Feature | What it does |
|---|---|
| **Project-scoped activation** | Global hooks only run when a project contains a valid `.mnemo` marker. |
| **MCP tools** | Agents can call `mem_save`, `mem_search`, `mem_context`, `mem_current_project`, `mem_doctor` and more. |
| **Session hooks** | Session start/end hooks register activity, inject context and capture learnings automatically. |
| **Portable Agent Skills** | Skills teach compatible agents when and how to use mnemo without falling back to native memory. |
| **Passive capture** | Extracts useful learnings from transcripts and subagent output. |
| **Agent provenance** | Records SQL-queryable agent, source, tool, model and MCP client metadata for writes that provide it. |
| **Diagnostics** | `mnemo doctor` checks project activation, global setup, MCP, hooks, competing memory surfaces and database migration health. |
| **Database safety** | Safe schema migrations run automatically; `mnemo db migrate --check` validates the local store for CI or troubleshooting. |
| **Project maintenance** | `mnemo projects list`, `mnemo projects merge` and `mnemo projects rename` help curate duplicate or unclear project identities. |
| **Memory curation** | `mnemo memories review` surfaces duplicate or conflicting observations for approved repair. |

## Supported Agents

<p align="center">
  <img alt="Claude Code" src="https://img.shields.io/badge/Claude%20Code-supported-6B46C1?logo=claudecode&logoColor=white">
  <img alt="Codex" src="https://img.shields.io/badge/Codex-supported-00A67E?logo=openai&logoColor=white">
  <img alt="Cursor" src="https://img.shields.io/badge/Cursor-supported-111111?logo=cursor&logoColor=white">
  <img alt="Windsurf" src="https://img.shields.io/badge/Windsurf-supported-2563EB?logo=windsurf&logoColor=white">
  <img alt="OpenCode" src="https://img.shields.io/badge/OpenCode-supported-F97316?logo=opencode&logoColor=white">
  <img alt="fx" src="https://img.shields.io/badge/fx-supported-7C3AED?logo=vercel&logoColor=white">
  <img alt="Pi" src="https://img.shields.io/badge/Pi-supported-0EA5E9">
</p>

| Agent | MCP | Hooks / runtime | Global instructions | Skill access | Status |
|---|---:|---:|---:|---:|---|
| Claude Code | ✅ | Plugin or n/a via `install.sh` | ✅ | ✅ | Supported |
| Codex | ✅ | ✅ | ✅ | ✅ | Supported |
| Cursor | ✅ | ✅ | ✅ | ✅ | Supported |
| Windsurf | ✅ | ✅ | ✅ | ✅ | Supported |
| OpenCode | ✅ | ✅ | ✅ | ✅ | Supported |
| fx | ✅ | n/a | ✅ | ✅ via canonical path | Supported |
| Pi | ✅ via MCP extension | n/a | ✅ | ✅ via `~/.pi/agent/skills/` | Supported |

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
| Auto installer | You want the binary plus detected agent setup | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; bash</code> |
| Explicit agent | You only want one integration | `bash -s -- --agent=codex` |
| All agents | You want every supported integration prepared | `bash -s -- --agent=all` |
| Claude plugin | You use Claude Code's plugin marketplace | `claude plugin install mnemo@mnemo` |
| Source build | You develop mnemo itself | `go build -o ~/.local/bin/mnemo ./cmd/mnemo/` |

Read the complete setup guide in [docs/INSTALLATION.md](docs/INSTALLATION.md).

### Codex hook review

Codex protects every hook in `~/.codex/hooks.json` with an interactive trust
review. mnemo currently installs Codex `SessionStart` and `Stop` hooks; the same
Codex trust mechanism will also apply whenever a mnemo-owned hook is added or
its command changes.

If Codex says a mnemo hook needs review, or if `mnemo setup status --agent=codex`
shows the Codex `Hooks` column as `no`, open Codex normally and approve the
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
| [Documentation index](docs/README.md) | Full documentation map and research notes. |
| [Installation](docs/INSTALLATION.md) | Install script, plugin setup, project activation and verification. |
| [Agent integration](docs/AGENT_INTEGRATION.md) | Hook behavior, global paths, `.mnemo` marker and Agent Skills. |
| [CLI reference](docs/CLI.md) | Commands, examples, MCP tools and search modes. |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | `doctor`, `setup status`, manual checks and idempotency validation. |
| [Storage](docs/STORAGE.md) | SQLite location, schema notes and sqlc workflow. |
| [Roadmap](ROADMAP.md) | Planned product and maintenance work. |

## Design principles

- **Local-first:** memory stays on your machine in SQLite.
- **Agent-neutral:** one memory authority across supported coding agents.
- **Opt-in by project:** global integrations are inert without `.mnemo`.
- **Diagnosable:** every setup surface can be checked without mutation.
- **Repairable:** duplicate project identities and memory conflicts are visible and fixable through CLI primitives.

## License

[Apache 2.0](LICENSE): you may use, modify, and distribute freely, but must retain the copyright notice and include the [NOTICE](NOTICE) file in all distributions.
