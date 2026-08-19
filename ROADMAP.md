# Roadmap

This document tracks planned capabilities that are not yet released. Released behavior belongs in release notes, not here.

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

## Agent trace

Add optional project trace metadata related to agent sessions and command execution.

Goals:

- relate commands and actions to a project and session;
- record commands executed for each project;
- use trace data to improve passive capture and debugging.

## Architecture alignment

Reduce operational drift between installation, setup, schema evolution, and CLI composition.

Near-term goals:

- unificar el escritor de setup para que el binario sea la única autoridad sobre MCP, hooks, instrucciones globales y runtime files;
- unificar la fuente de verdad del esquema para que sqlc y las migraciones no evolucionen por caminos paralelos;
- `cmd/mnemo` separa memoria, init/migrate, MCP, setup, doctor, projects y utilidades operativas; `main.go` solo enruta;
- `internal/agentinit` posee la lógica por agente (rutas, snippets, runtime, uninstall y doctor). `cmd/mnemo` orquesta CLI y no vuelve a ramificar por agente.

## Later-stage ideas

- terminal UI for memory browsing and curation;
- optional cloud replication after local sync is mature;
- Obsidian/Markdown export for human review.
