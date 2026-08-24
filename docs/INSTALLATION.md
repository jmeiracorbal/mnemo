# Installation

mnemo uses a CodeGraph-style split:

1. Install the `mnemo` binary and global agent setup once.
2. Run `mnemo init` only in projects that should use mnemo.

Global hooks are intentionally inert outside projects with a valid `.mnemo` marker.

## Prerequisite: binary in PATH

The `mnemo` binary must be in your `PATH` before any agent integration will work. This applies to Claude Code, Cursor, Windsurf, Codex and OpenCode regardless of how the integration is installed.

The hooks that fire on session start, session end and passive capture call `mnemo` directly. The MCP server is also the `mnemo` binary. Without it in PATH, hooks and MCP cannot start.

## Install from the release installer

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
```

The installer defaults to `--agent=auto`: it detects compatible agents from known binaries/directories and falls back to Claude Code when nothing is detected.

Explicit targets are supported:

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=codex
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=opencode
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash -s -- --agent=all
```

Environment overrides:

```bash
MNEMO_AGENT=cursor bash install.sh
MNEMO_VERSION=v0.31.0 bash install.sh
MNEMO_DRY_RUN=true bash install.sh
MNEMO_INSTALL_DIR="$HOME/.local/bin" bash install.sh
```

## Claude Code plugin

Claude Code users may install through the plugin marketplace:

```bash
claude plugin marketplace add jmeiracorbal/mnemo
claude plugin install mnemo@mnemo
```

The plugin still requires the `mnemo` binary in PATH. Restart Claude Code after installing, then run:

```bash
mnemo init --agent=claudecode
```

Update the plugin with:

```bash
claude plugin update mnemo@mnemo
```

If that reports up to date while the binary is newer, reinstall to clear marketplace cache:

```bash
claude plugin uninstall mnemo
claude plugin install mnemo@mnemo
```

`install.sh` configures Claude Code through MCP and global instructions. Claude plugin hooks are optional in that path, so a fresh install without a plugin registry is valid.

## Build from source

```bash
git clone https://github.com/jmeiracorbal/mnemo
cd mnemo
go build -o ~/.local/bin/mnemo ./cmd/mnemo/
export PATH="$HOME/.local/bin:$PATH"
mnemo --version
```

## Activate a project

From the project root:

```bash
mnemo init --agent=claudecode   # or cursor, windsurf, codex, opencode, all
# optional: --no-project-rules for .mnemo marker only
```

This creates a `.mnemo` marker and records the selected agent. Agent hooks, MCP configuration and global protocol live globally after installation; they activate only when a project has a valid `.mnemo` file.

## Optional skills

`install.sh` and `mnemo setup refresh` embed the `mnemo-memory` skill at `~/.agents/skills/mnemo-memory/` and link Claude Code and Windsurf to it. You normally do not need a separate skill install step.

If you prefer the skills CLI or need to refresh manually:

```bash
npx skills add jmeiracorbal/mnemo --skill mnemo-memory --global
```

Optional project maintenance skill:

```bash
npx skills add jmeiracorbal/mnemo --skill mnemo-project-maintenance --global
```

This installs only the skill and agent links. It does not install the `mnemo` binary, hooks, MCP configuration or Claude Code plugin. Install mnemo first and ensure `mnemo` is available in PATH.

Optional memory curation skill:

```bash
npx skills add jmeiracorbal/mnemo --skill mnemo-memory-curation --global
```

Use it when an agent should notice likely memory conflicts during normal work, warn the user and apply only explicitly approved repairs through mnemo commands.

## Setup lifecycle commands

```bash
mnemo setup status --agent=all
mnemo setup print-config codex
mnemo setup refresh --agent=codex
mnemo setup uninstall --agent=codex
```

`mnemo setup <agent>` is an alias for `mnemo setup refresh --agent=<agent>`.
