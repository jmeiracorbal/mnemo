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

## Skill agent metadata coverage

Add agent-specific metadata coverage for every shipped skill after the project maintenance skill lands.

Goals:

- decide which `agents/` metadata files are required per supported agent and keep them consistent across all skills;
- verify whether each agent detects metadata files, symlinked skill folders, or only real skill directories;
- prefer canonical skill folders as the source of truth, using symlinks only for installed global agent locations when agents reliably support them;
- update validation so new skills cannot ship with incomplete agent metadata.

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
