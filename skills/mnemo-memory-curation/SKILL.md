---
name: mnemo-memory-curation
description: Detect, report, and safely repair mnemo memory conflicts. Use when Codex notices contradictory decisions, duplicate memories, stale patterns, overlapping topic keys, or when proactive memory hygiene is needed. Default to read-only review, notify the user about likely conflicts, and apply repairs only with explicit approval through official mnemo commands.
---

# mnemo Memory Curation

Use mnemo CLI commands to find and repair conflicting or duplicated memories. This skill is independent from `mnemo-memory`; use `mnemo-memory` separately when the task also involves normal context recovery or saving memories.

## Safety rules

- Never edit `~/.mnemo/memory.db` or other mnemo store files directly.
- Never use native agent memory, `MEMORY.md`, arbitrary plaintext files, or alternative stores for repair notes.
- Default to read-only review commands.
- Treat detected conflicts as candidates, not facts, until reviewed.
- Notify the user when likely conflicts are found during normal work; do not wait only for an explicit review request.
- Ask for explicit approval before running any command that mutates memories.
- Use only official `mnemo memories ...` commands for repairs.

## Proactive triggers

Run a read-only review when any of these happen:

- retrieved context contains incompatible decisions or conventions;
- two memories appear to describe the same topic with different current guidance;
- a memory looks superseded by newer work;
- repeated titles, duplicate content, or overlapping topic keys make the next action ambiguous;
- the user asks to clean, review, consolidate, or repair memory quality.

## Workflow

1. Verify the CLI supports memory review:

   ```bash
   mnemo version
   ```

   If the version is older than the release that introduced `mnemo memories`, ask the user to update mnemo before planning repairs.

2. Run a read-only review, scoped when possible:

   ```bash
   mnemo memories review --json
   mnemo memories review --project=PROJECT --json
   mnemo memories review --topic=TOPIC_KEY --json
   ```

3. Interpret the report:

   - `duplicate-content` usually means one memory should become canonical and others should be marked stale or superseded.
   - `topic-conflict` means multiple live memories share a topic key and may conflict.
   - `duplicate-title` means memories may be related but require human review before repair.
   - Prefer newer, more specific memories as canonical, but do not assume recency alone is enough.

4. Notify the user concisely when conflicts are found:

   - summarize candidate count and confidence;
   - explain why the conflict matters for the current work;
   - propose the safest follow-up;
   - ask before mutating memory.

5. Apply only approved repairs:

   ```bash
   mnemo memories mark-reviewed OBSERVATION_ID --reason=TEXT
   mnemo memories mark-stale OBSERVATION_ID --reason=TEXT
   mnemo memories supersede OLD_ID --by=NEW_ID --reason=TEXT
   mnemo memories consolidate-topic --from=OLD_TOPIC --to=NEW_TOPIC --dry-run --json
   mnemo memories consolidate-topic --from=OLD_TOPIC --to=NEW_TOPIC --yes
   ```

6. Verify after repair:

   ```bash
   mnemo memories review --json
   ```

   Confirm that resolved items no longer appear, and report any remaining ambiguous candidates.

7. Record the repair through `mem_save` by using `mnemo-memory` when memory tools are available and the current project has a valid `.mnemo` marker. The record must include what was repaired, why it was safe, and any remaining follow-up.
