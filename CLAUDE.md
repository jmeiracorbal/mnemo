# CLAUDE.md

## Purpose

This repository contains two separate but related deliverables:

1. The `mnemo` Go binary.
2. The Claude Code plugin metadata and hooks.

They are intentionally separate.
The plugin cannot replace the binary setup flow.
Do not "simplify" the architecture by merging responsibilities that are separated on purpose.

## Non-negotiable project rules

### 1. Do not confuse plugin responsibilities with binary responsibilities

The Claude plugin is responsible for plugin metadata, hooks, and MCP-related integration.

The Go binary is responsible for setup tasks that the plugin cannot perform reliably by itself, including user environment setup and file modifications such as include/protocol installation.

Never report the separation between binary and plugin as a design bug by default.
Only flag issues when both parts become inconsistent with each other.

### 2. Before changing hooks, verify real script names and paths

When editing or reviewing hook definitions:

- verify the exact filename exists
- verify the hook path matches the actual shipped script
- verify naming consistency between:
  - `plugin/claude-code/hooks/hooks.json`
  - shipped plugin script paths
  - installer/setup code
  - tests

Do not assume similar names are interchangeable.

Known example of a prior failure:
- `post-compaction.sh` was referenced in hook config
- real script name was `post-compact.sh`

This class of error must be treated as critical.

### 3. Never change version references partially

When preparing a new version, do not stop after updating only one file.
Search the repository for the exact current version and update every required release metadata location.

At minimum, verify these files:

- `.claude-plugin/marketplace.json`
- `.claude-plugin/plugin.json`
- `plugin/claude-code/.claude-plugin/plugin.json`

Also verify that the release tag matches the intended versioning scheme used by CI/release automation.

Before considering a version bump complete:
- search for the old version string across the full repository
- confirm no stale copies remain in release metadata
- confirm the binary version source still aligns with release tagging

### 4. Treat version metadata as a consistency boundary

The binary version and plugin version metadata must not silently drift.

If a release changes version metadata:
- verify plugin metadata files are aligned
- verify release workflow behavior is aligned
- verify install/update paths still resolve the correct published version

If any of those are inconsistent, report it explicitly.

### 5. Keep documentation accurate about what the plugin really does

Do not state that the plugin performs actions that are actually done by the binary setup flow.

When editing README or installation docs:
- distinguish clearly between binary installation
- plugin installation
- hook registration
- setup-side file modifications
- MCP/profile configuration

If the plugin depends on the binary being installed and available in `PATH`, state that explicitly and near the plugin installation steps.

### 6. Project identity derivation must stay consistent across hooks

Project identity derivation is shared behavior and must remain consistent across all hooks.

Current expected behavior:
- if the current working directory is inside `HOME`, derive the project name from the full path relative to `HOME`
- if the current working directory is outside `HOME`, derive it from the current working directory location

If one hook derives `PROJECT` differently from another, that is a bug.

When touching any hook or setup logic that derives project identity:
- compare all related scripts
- preserve the same derivation strategy everywhere
- preserve the same normalization rules everywhere
- avoid inconsistent fallback logic that changes the resulting key shape between execution contexts beyond the defined HOME-relative / CWD fallback behavior

Any change in project identity derivation must be treated as a storage compatibility change, because it can fragment or orphan stored memory/context for the same project.

### 7. Tests must validate the shipped plugin, not only installer internals

When adding or updating tests, ensure they cover actual user-facing deliverables.

Do not stop at testing embedded installer assets.
Also validate:
- plugin hook configuration references existing script files
- shipped plugin metadata is internally consistent
- expected filenames in hook JSON match actual filenames
- release metadata files stay aligned

If a real shipped file can be wrong while tests still pass, test coverage is insufficient.

## Functional verification before commit

Changes must not be considered complete just because the code builds or unit tests pass.

For this repository, every change affecting hooks, plugin metadata, setup/install flows, versioning, or memory behavior must be functionally verified before commit.

### 8. Run functional checks before every commit that touches behavior

If a change affects any of the following:

- hook scripts
- `plugin/claude-code/**`
- `.claude-plugin/**`
- setup/install code
- release/version metadata
- project identity derivation
- memory persistence or restore flows
- README installation or usage instructions

you must perform functional verification before committing.

Do not rely only on static review.

### 9. Minimum pre-commit verification standard

Before committing a behavioral change, verify all that apply:

- the project builds successfully
- relevant automated tests pass
- the affected hook or setup flow works end-to-end
- changed documentation still matches actual behavior
- no previously working path was broken by the fix

A fix is incomplete if it solves the reported issue but breaks:
- another hook
- another install path
- version consistency
- project identity consistency
- restore/persistence behavior
- plugin validation

### 10. Test the user-visible workflow, not only isolated code

For this repository, functional validation should prioritize real user flows.

Examples:
- if a hook definition changes, validate that the hook can actually execute the referenced script
- if a script filename changes, validate every place that references it
- if setup logic changes, validate the installed result, not only the internal function
- if version metadata changes, validate all published metadata files together
- if project identity logic changes, validate that the same project resolves to the same key across all related hooks
- if README installation steps change, validate that the documented flow still works as written

### 11. Required mindset for fixes

When fixing a bug, do not stop after confirming the original bug is gone.

Also verify:
- adjacent flows
- reverse flows
- fallback paths
- reinstall/update scenarios
- previously supported paths

Every bug fix must include regression thinking.

### 12. Prefer small verification matrices for risky changes

For changes that touch shared behavior, explicitly verify the main execution contexts.

At minimum, cover the relevant combinations such as:
- binary setup path
- plugin-installed path
- hook execution path
- fresh install
- existing install/update
- inside `HOME`
- outside `HOME`

Do not assume one successful path proves the others.

### 13. No commit without stating what was verified

For behavioral changes, the commit or PR description must state what was actually verified.

Include concise verification notes such as:
- build completed
- tests passed
- plugin hook path validated
- setup flow validated
- version metadata aligned
- project identity behavior checked in `HOME` and outside `HOME`

Do not claim completion without verification evidence.

### 14. Patch and minor releases must be regression-sensitive

Patch and minor releases must not be treated as safe by default.

Even a small release may break:
- hook wiring
- plugin metadata
- install flows
- memory restore
- version coherence

For patch and minor changes:
- verify the exact fix
- verify nearby behavior
- verify release metadata consistency
- verify user-facing installation/use paths still work

### 15. If functionality was not verified, say so explicitly

If full functional verification could not be performed, do not imply confidence that has not been earned.

State clearly:
- what was verified
- what was not verified
- what remains at risk

Never present an unverified behavioral change as complete.

## Required workflow before making changes

Before modifying any of these areas:
- release versioning
- plugin metadata
- hook configuration
- setup/install behavior
- README installation docs

you must do all of the following:

1. Read the relevant files completely.
2. Search the repository for duplicated references.
3. Identify all sources of truth and all derived artifacts.
4. State explicitly which files must remain aligned.
5. Only then apply changes.

## Required workflow before opening a PR

For any PR that affects versions, hooks, setup, plugin metadata, install docs, or runtime behavior, verify:

- hook filenames are exact
- hook paths exist
- status messages describe the real action being executed
- version strings are updated in every required metadata file
- release/tag behavior remains coherent
- README language matches actual behavior
- project identity derivation remains consistent
- tests cover shipped plugin artifacts, not just internal installer data
- functional behavior was validated in the affected user flow
- the fix did not break adjacent or previously working paths

## Review checklist

Use this checklist in every relevant task.

### Hook changes
- Is every referenced script real?
- Do filenames match exactly?
- Do hook messages match actual behavior?
- Are plugin config and setup logic aligned?

### Version bumps
- Did I search for the previous version across the repository?
- Did I update all required plugin metadata files?
- Does release automation still derive the correct binary version?
- Is there any stale version left in release-facing metadata?

### Documentation changes
- Does the text distinguish plugin vs binary responsibilities correctly?
- Does it avoid claiming the plugin performs setup-only actions?
- Are prerequisites stated close to the relevant install step?

### Identity/storage changes
- Is `PROJECT` computed consistently everywhere?
- Could this change fragment stored memory/context?
- Are fallback paths normalized identically?

### Testing
- Do tests validate what users actually install?
- Can hook/script mismatches still slip through?
- Are metadata alignment checks covered?

### Functional verification
- Did I test the real affected workflow end-to-end?
- Did I verify both the direct fix and nearby paths?
- Did I test the shipped artifact, not only internal code?
- Did I verify update/reinstall implications if relevant?
- Am I sure this patch/minor change does not introduce a new regression?

## Preferred behavior when analyzing this repository

When you review this project:
- be conservative with architectural criticism
- do not invent simplifications that break the binary/plugin separation
- focus on concrete inconsistencies
- prioritize exactness over broad refactors
- prefer repository-wide verification over local assumptions

## Anti-patterns to avoid

Do not:
- claim the binary/plugin split is inherently wrong
- rename scripts without checking all references
- update only one version file
- trust tests that ignore shipped plugin files
- change docs without checking actual implementation
- introduce a second way to derive project identity
- assume release metadata is centralized if it is not

## Expected output style for this repository

When reporting issues or proposing fixes:
- list exact files
- describe the concrete inconsistency
- explain user impact
- propose the minimal correct fix
- distinguish bug, documentation issue, and design issue

Avoid vague statements such as:
- "there is duplication"
- "this should be simplified"
- "the plugin should do everything"

Be precise.