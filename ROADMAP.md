# Roadmap

This document tracks planned capabilities that are not yet released. Released behavior belongs in release notes, not here.

## Project management safety flows

Build on project inventory and consolidation before introducing more destructive cleanup flows:

```bash
mnemo projects prune
mnemo projects rename --id <project> --name <name>
mnemo projects rename --path <dir> --name <name>
```

Near-term goals:

- keep `prune` behind explicit safety checks after merge/consolidation is reliable;
- revisit `rename` after consolidation, using explicit selectors (`--id`, `--path`, or a shared selector flag) instead of ambiguous positional arguments.

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

Add a separate, explicitly invoked Agent Skill for memory maintenance. It should use admin and tag-management tools without expanding the normal `mnemo-memory` workflow.

Planned responsibilities:

- inspect memory statistics and timelines;
- identify duplicate, stale, or low-value observations;
- merge inconsistent tags and review tag usage;
- consolidate evolving topics safely;
- propose deletions before performing destructive operations;
- produce a concise curation report.

The skill must require a valid `.mnemo` marker, default to read-only analysis, and request explicit confirmation before deletion or broad mutation.

## Memory conflicts and review

Add higher-level review tools for memory quality:

- compare possibly related memories;
- surface contradictory decisions or stale patterns;
- mark observations as reviewed;
- identify topic keys that should be consolidated.

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
- repartir mejor las responsabilidades de `cmd/mnemo`, separando comandos de memoria, setup, diagnóstico y utilidades operativas;
- `internal/agentinit` posee la lógica por agente (rutas, snippets, runtime, uninstall y doctor). `cmd/mnemo` orquesta CLI y no vuelve a ramificar por agente.

## Later-stage ideas

- terminal UI for memory browsing and curation;
- optional cloud replication after local sync is mature;
- Obsidian/Markdown export for human review.
