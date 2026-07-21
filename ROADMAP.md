# Roadmap

This document tracks planned capabilities that are not yet released. Released behavior belongs in release notes, not here.

## Diagnostics

### mnemo setup status

Add a global setup status command that reports detected and configured agent integrations without performing repairs.

Target output shape:

```text
Agent       Detected  MCP  Hooks  Instructions
Claude      yes       yes  yes    yes
Codex       yes       yes  yes    yes
Cursor      yes       yes  yes    yes
OpenCode    no        no   no     no
```

## Setup lifecycle

Add explicit setup maintenance commands for global agent integrations:

```bash
mnemo setup refresh --agent=all
mnemo setup uninstall --agent=codex
mnemo setup print-config codex
```

Goals:

- refresh previously installed global files after upgrades;
- uninstall mnemo from selected agents without removing the local database;
- print target-specific MCP/config snippets for manual setup and debugging.

## Project management

Add project management commands for UUID-based projects:

```bash
mnemo projects list
mnemo projects prune
mnemo projects merge <from> <to>
mnemo projects rename <id> <name>
```

Goals:

- make old or unused project records visible;
- provide safe cleanup for stale projects;
- support explicit consolidation when project identity changes.

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

## Later-stage ideas

- terminal UI for memory browsing and curation;
- optional cloud replication after local sync is mature;
- Obsidian/Markdown export for human review.
