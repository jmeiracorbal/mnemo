# Roadmap

This document tracks planned capabilities that are not yet released. Released behavior belongs in release notes; brief "already applied" notes are kept only when they prevent re-planning completed work.

## Project management safety flows

Build on project inventory and consolidation before introducing more destructive cleanup flows:

```bash
mnemo projects prune
```

Near-term goals:

- keep `prune` behind explicit safety checks after merge/consolidation is reliable;
- continue using explicit selectors and dry-run/apply guardrails for project maintenance commands.

## Local sync

Expose local/Git-friendly sync flows before introducing any cloud replication:

```bash
mnemo sync export
mnemo sync import
mnemo sync status
```

Goals:

- share project-scoped memory across machines through version-controlled chunks;
- keep local SQLite as the source of truth;
- avoid merge conflicts and large generated files.

## Memory curation

Build on memory conflict review with deeper, explicitly invoked maintenance flows. These should use explicit mnemo maintenance commands without expanding the normal `mnemo-memory` workflow or requiring agents to edit the store directly.

Planned responsibilities:

- inspect memory statistics and timelines;
- identify low-value observations beyond duplicate or conflicting memories;
- merge inconsistent tags and review tag usage;
- propose deletions before performing destructive operations;
- produce broader curation reports across projects, tags, and time ranges.

The skill workflow must require a valid `.mnemo` marker, default to read-only analysis, and request explicit confirmation before deletion or broad mutation.

## Static agent memory migration

Help users move from static agent memory files to mnemo as the canonical memory store, so agentic systems stop accumulating heavy `MEMORY.md`, multi-file Claude memories, editor rules, or other plaintext memory surfaces.

Planned capabilities:

- detect competing or oversized memory files such as `MEMORY.md`, Claude memory files, Cursor/Windsurf rules used as memory, and other static memory surfaces;
- provide a dry-run importer that parses candidate memories into mnemo observations before writing anything;
- preserve provenance for imported memories, including source file path, heading or section, import timestamp, and confidence;
- deduplicate imported chunks against existing mnemo observations;
- classify imported memories as decision, bugfix, discovery, pattern, config, preference, or manual;
- keep instructions separate from memory, avoiding blind imports of agent operating instructions;
- after approved import, replace heavy memory files with a minimal mnemo authority stub or archive them only by explicit user choice;
- extend `mnemo doctor` to warn about competing static memory files and suggest the migration flow;
- add an Agent Skill that can detect static-memory drift, propose a dry-run migration, request approval, run the approved import, and repair the project to use mnemo as the only memory store.

Potential commands:

```bash
mnemo memories ingest --path MEMORY.md --dry-run --json
mnemo memories ingest --agent=claudecode --path . --dry-run --json
mnemo memories ingest --agent=all --path . --yes
```

## Harness integration contract

mnemo should not claim universal memory across arbitrary agent harnesses. It is a local memory substrate with first-class support for integrated agents, and broader adoption requires a clear, low-friction contract that custom harnesses can implement and verify.

Goals:

- define the minimum integration contract for custom harnesses: project marker discovery, MCP or CLI write/read paths, session lifecycle events, passive capture, and explicit `.mnemo` guardrails;
- provide copy-paste integration examples for MCP-first, CLI-only, hook-based, and skill-based harnesses;
- add conformance checks so harness authors can prove their integration respects project identity, does not write outside valid `.mnemo` projects, and records provenance when available;
- document supported versus custom-integrated harnesses without marketing mnemo as automatic memory for every runtime;
- consider a `mnemo harness check` command or fixture suite that validates environment variables, MCP server configuration, hook behavior, and write/read smoke tests.

## Agent trace and memory provenance

Add optional project trace metadata related to agent sessions, command execution, and memory provenance.

Status:

- completed: normalized SQLite provenance catalog for agents, source kinds, tools, models, MCP clients, and contexts;
- completed: nullable `provenance_id` links for sessions, observations, and prompts, preserving legacy rows;
- completed: CLI/MCP/import write paths can persist provenance when available;
- next: enforce `.mnemo` marker guardrails in MCP write-tool contracts so direct callers cannot write to an arbitrary `project` without a validated project marker;
- next: expose user-facing filters and reports such as `--agent`, `--source`, and stats by agent.

Goals:

- relate commands and actions to a project and session;
- record commands executed for each project;
- record which agent and source created each session, prompt, passive capture, and observation;
- distinguish memories created through CLI, MCP, hooks, skills, imports, and passive capture;
- keep provenance metadata separate from project identity, which remains based on the `.mnemo` marker id;
- expose filters and reports such as `--agent`, `--source`, and stats by agent to support review, trust, and debugging;
- use trace and provenance data to improve passive capture, curation, and conflict review.

Potential metadata fields:

```txt
agent: codex | claudecode | cursor | windsurf | opencode | cli | unknown
source: mcp | cli | hook | passive_capture | import | skill
tool: mem_save | capture | session_end | ingest | ...
model: optional model identifier when available
```

## Architecture alignment

Reduce operational drift between installation, setup, schema evolution, and CLI composition.

Already applied and no longer pending:

- setup writes are delegated to the `mnemo` binary through `mnemo setup refresh`, which owns MCP config, hooks, global instructions, and runtime files;
- per-agent expectations are centralized in `internal/agentinit.AgentSpec`, covering paths, MCP snippets/checks, hooks, skills, install, uninstall, status, and doctor-facing checks;
- `cmd/mnemo` is split by command domain, with `main.go` acting as the router;
- `internal/agentinit` owns per-agent setup logic, including routes, snippets, runtime assets, uninstall, and diagnostic checks.

Remaining near-term goals:

- replace inline runtime schema evolution with a versioned SQL migration plan:
  - keep a canonical current schema for new databases and `sqlc` generation;
  - store structural changes as ordered migration scripts under `database/migrations/`, for example `0001-create-schema-database.sql`, `0002-add-new-column-example.sql`, and so on;
  - make `store.go` responsible for migration orchestration only: discover applied migrations, detect pending scripts, apply them in order, and fail loudly when the database is ahead, inconsistent, or missing required fields;
  - avoid balancing DDL or data-shape changes between migration scripts and ad hoc runtime queries;
- keep future setup/status/doctor changes flowing through `AgentSpec` so installers, diagnostics, and tests do not drift again.

## Later-stage ideas

- terminal UI for memory browsing and curation;
- optional cloud replication after local sync is mature;
- Obsidian/Markdown export for human review.
