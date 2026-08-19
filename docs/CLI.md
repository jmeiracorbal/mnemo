# CLI reference

## Commands

```text
mnemo mcp [--tools=PROFILE]          Start MCP server (stdio)
mnemo init [--agent=AGENT] [--no-project-rules]  Activate mnemo in the current project (.mnemo)
mnemo install-instructions [--agent=AGENT]  Install global agent instructions
mnemo doctor [--json] [--agent=AGENT] [--path=DIR]  Run read-only diagnostics
mnemo setup status [--json] [--agent=AGENT] [--home=DIR]  Show global agent setup status
mnemo setup print-config AGENT [--home=DIR] [--mnemo-bin=PATH]  Print manual setup config snippets
mnemo setup refresh [--agent=AGENT] [--home=DIR] [--mnemo-bin=PATH]  Refresh installed global setup files
mnemo setup AGENT [--home=DIR] [--mnemo-bin=PATH]  Alias for setup refresh --agent=AGENT
mnemo setup uninstall --agent=AGENT [--home=DIR]  Remove global setup files for an agent
mnemo projects list [--sort=FIELD] [--asc|--desc] [--unused-since=DURATION|DATE] [--empty] [--json]  List known projects
mnemo projects merge --from=PROJECT --to=PROJECT (--dry-run|--yes) [--json]  Merge one project into another
mnemo projects merge --auto-by-path (--dry-run|--yes) [--json]  Merge duplicate project identities by shared directory
mnemo projects rename (--id=PROJECT|--path=DIR) --name=NAME (--dry-run|--yes) [--json]  Rename project display metadata
mnemo memories review [--project=PROJECT] [--topic=TOPIC_KEY] [--json]  Review potential memory conflicts
mnemo memories mark-reviewed OBSERVATION_ID [--reason=TEXT]  Mark a memory as reviewed
mnemo memories mark-stale OBSERVATION_ID [--reason=TEXT]  Mark a memory as stale
mnemo memories supersede OLD_ID --by=NEW_ID [--reason=TEXT]  Mark a memory as superseded by another
mnemo memories consolidate-topic --from=TOPIC --to=TOPIC (--dry-run|--yes) [--json]  Consolidate memory topic keys
mnemo save <title> <content>         Save a memory
mnemo search <query>                 Search memories
mnemo context [project]              Show context from previous sessions
mnemo session start <id>             Register session start
mnemo session end <id>               Mark session as completed
mnemo session exists <id>            Check if a session exists (exits 1 if not)
mnemo session obs-count <id>         Count observations saved in a session
mnemo stats                          Show memory statistics
mnemo export [file]                  Export all memories to JSON
mnemo import <file.json>             Import memories from JSON
mnemo capture <content>              Extract learnings from text (passive capture)
mnemo json <key> [key...]            Extract a field from JSON read from stdin (key path, array index supported)
mnemo extract-transcript <file>      Extract assistant text blocks from a JSONL transcript
mnemo version                        Show version
```

## Agent options

Agents for `mnemo init`, `mnemo install-instructions` and setup commands:

```text
--agent=claudecode   Claude Code global CLAUDE.md instructions / .mnemo activation
--agent=cursor       Cursor global rule / .mnemo activation
--agent=windsurf     Windsurf global rule / .mnemo activation
--agent=codex        Codex global AGENTS.md instructions / .mnemo activation
--agent=opencode     OpenCode global AGENTS.md instructions / .mnemo activation
--agent=all          All agents
```

## Examples

Save a decision manually:

```bash
mnemo save "Use FTS5 for search" "Chose SQLite FTS5 over external search" --type decision --project myapp
```

Search memories:

```bash
mnemo search "authentication" --project myapp --limit 5
```

Show context from previous sessions:

```bash
mnemo context myapp
```

Export everything to JSON:

```bash
mnemo export backup.json
```

List projects:

```bash
mnemo projects list --sort=last_seen --desc
mnemo projects list --unused-since=90d
mnemo projects list --json
```

Merge duplicate project identities by shared directory:

```bash
mnemo projects merge --auto-by-path --dry-run --json
mnemo projects merge --auto-by-path --yes
```

Rename project display metadata without changing the stable project ID:

```bash
mnemo projects rename --id=myapp --name="My App" --dry-run
mnemo projects rename --path=/path/to/myapp --name="My App" --yes
```

Review memory conflicts:

```bash
mnemo memories review --project=myapp
mnemo memories supersede 12 --by=18 --reason="newer decision is canonical"
mnemo memories consolidate-topic --from=architecture/auth --to=auth/model --dry-run
```

## MCP tools

Tools are available inside your editor through the `mcp__mnemo__*` namespace.

### Agent profile

| Tool | Description |
|---|---|
| `mem_save` | Save a memory with title, content, type, tags and optional topic key |
| `mem_search` | Search memories by text, tags, topic key or any combination |
| `mem_context` | Retrieve formatted context from previous sessions |
| `mem_session_summary` | Save an end-of-session summary with goal, discoveries and next steps |
| `mem_session_start` | Register a new session |
| `mem_session_end` | Mark a session as completed, optionally with tags |
| `mem_get_observation` | Retrieve full content of a memory by ID |
| `mem_suggest_topic_key` | Suggest a topic key for deduplication |
| `mem_capture_passive` | Extract and save learnings from free-form text |
| `mem_save_prompt` | Save a prompt template |
| `mem_update` | Update an existing memory, including tags |
| `mem_list_tags` | List all tags in use for a project, ordered by frequency |
| `mem_merge_tags` | Merge all occurrences of one tag into another |
| `mem_tag_stats` | Query tag observability: top tags, low-frequency tags, stale tags |
| `mem_related_tags` | Find tags that co-occur with a given tag across observations and sessions |
| `mem_current_project` | Resolve the current project identity from `.mnemo` |
| `mem_doctor` | Run read-only diagnostics from MCP |

### Admin profile

Available with `--tools=admin`:

- `mem_delete`
- `mem_stats`
- `mem_timeline`

To add the admin profile as a separate Claude MCP server:

```bash
claude mcp add -s user mnemo-admin -- ~/.local/bin/mnemo mcp --tools=admin
```

## Search modes

`mem_search` supports independent intents that compose:

| Parameter | Type | Semantics |
|---|---|---|
| `query` | text, optional | Full-text search via FTS5. Omit to browse by other filters. |
| `tags` | comma list | Hard filter. Only observations that have **all** listed tags are returned. |
| `prefer_tags` | comma list | Soft signal. Observations matching more tags rank higher. Non-matching results are still returned. |
| `topic_key` | string | Hard filter. Only observations with this exact topic key. |
| `type` | string | Hard filter by observation type, e.g. `decision`, `bugfix`. |
| `project` | string | Scope to a project. |

`mem_context` accepts `tags`, `prefer_tags` and `topic_key` with the same semantics, applied to recent observation retrieval.
