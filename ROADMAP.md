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

## Project maintenance skill

Add a separate Agent Skill for project inventory maintenance so agents can diagnose and resolve common project identity problems without requiring the user to manually inspect IDs and run every merge command.

Planned responsibilities:

- run `mnemo projects list --json` and identify likely duplicates by shared directory/path, legacy key patterns, and UUID project metadata;
- produce a concise project maintenance report with proposed merge groups and destination projects;
- execute high-confidence merge plans through `mnemo projects merge` when explicitly requested or when a user has opted into automatic project maintenance;
- fall back to asking for confirmation when merge targets are ambiguous, when data would be moved across unrelated paths, or when destructive cleanup is involved;
- record what was merged and why, so future agents understand previous consolidation decisions.

The skill should be read-only by default, support dry-run planning, and require clear opt-in before broad mutation.

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
