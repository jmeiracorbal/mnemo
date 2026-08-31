---
name: mnemo-project-maintenance
description: Diagnose and safely repair duplicate mnemo project identities. Use when Codex needs to inspect `mnemo projects list`, find duplicate or stale project IDs, plan or apply `mnemo projects merge`, consolidate same-project records across UUID, path-derived, legacy, renamed, or moved directories, or perform project inventory maintenance. Default to read-only dry-run analysis and require explicit user approval before any `--yes` merge.
---

# mnemo Project Maintenance

Use mnemo CLI commands to find and consolidate duplicate project identities. This skill is independent from `mnemo-memory`; use `mnemo-memory` separately only when the task also involves persistent memory.

## Safety rules

- Never edit `~/.mnemo/memory.db` or other mnemo store files directly.
- Default to read-only commands: `mnemo projects list --json` and `mnemo projects merge --auto-by-path --dry-run --json`.
- Do not run `mnemo projects merge ... --yes` unless the user explicitly approves applying the exact or described plan.
- Ask for confirmation when paths, names, or merge targets are ambiguous.
- Treat `mnemo projects merge --auto-by-path` without `--dry-run` or `--yes` as an intentional guardrail, not a broken command.

## Workflow

1. Verify the installed CLI supports project maintenance:

   ```bash
   mnemo version
   ```

   If the version is older than `v0.26.0` or the binary is missing, ask the user to run `mnemo update` before planning merges.

2. Build the project inventory:

   ```bash
   mnemo projects list --json
   ```

3. Generate the safe automatic merge plan:

   ```bash
   mnemo projects merge --auto-by-path --dry-run --json
   ```

4. Review the JSON results before suggesting changes:

   - `from` is the duplicate/source project that would be absorbed.
   - `to` is the canonical destination project that would keep the consolidated memories.
   - Include only completed dry-run results in the proposed plan.
   - If the command reports an error with partial results, show the successful results and the error separately.

5. Classify candidates:

   - **High confidence:** same normalized directory/path; UUID project plus legacy/path-derived project for the same directory; CLI auto plan clearly maps a stale duplicate into an active destination.
   - **Medium confidence:** same repository name in different directories, likely moved project, or path plus ID records that appear related but not identical.
   - **Low confidence:** similar names only, missing directory evidence, temporary/personal paths, or unrelated repositories.

6. Report concisely:

   - summarize candidate counts;
   - list high-confidence `from -> to` merges first;
   - mark medium/low-confidence items as requiring confirmation or manual review;
   - show the command that would apply the approved plan, but do not execute it yet.

7. Apply only after explicit opt-in:

   - Use `mnemo projects merge --from=PROJECT --to=PROJECT --yes` for individually approved merges.
   - Use `mnemo projects merge --auto-by-path --yes` only when the user explicitly approves the full dry-run plan.
   - If a later merge fails, preserve and report earlier successful merge results before showing the error.

8. Verify after applying:

   ```bash
   mnemo projects merge --auto-by-path --dry-run --json
   mnemo projects list --json
   ```

   Confirm that the applied duplicates no longer appear and call out any remaining ambiguous candidates.

9. Record the repair through `mem_save` by using `mnemo-memory` when memory tools are available and the current project has a valid `.mnemo` marker. The record must include what was merged, why it was safe, and any remaining follow-up. Never use native agent memory, `MEMORY.md`, arbitrary plaintext files, or alternative stores.
