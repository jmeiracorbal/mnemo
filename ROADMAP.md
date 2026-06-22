# Roadmap

This file tracks planned capabilities that are not yet released. Released behavior belongs in release notes, not here.

## mnemo-curate

Add a separate, explicitly invoked Agent Skill for memory maintenance. It should use the admin and tag-management tools without expanding the normal `mnemo-memory` workflow.

Planned responsibilities:

- inspect memory statistics and timelines;
- identify duplicate, stale, or low-value observations;
- merge inconsistent tags and review tag usage;
- consolidate evolving topics safely;
- propose deletions before performing destructive operations;
- produce a concise curation report.

The skill must require a valid `.mnemo` marker, default to read-only analysis, and request explicit confirmation before deletion or broad mutation.

## mnemo-trace

Add project trace related with the agent session before action will be excecuted to inspect that commands will be related with a agent's actions.

- relate commands and actions with a project.
- commands executed for every project.
