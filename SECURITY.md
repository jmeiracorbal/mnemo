# Security Policy

## Supported versions

Security fixes target the latest published stable release on
[GitHub Releases](https://github.com/jmeiracorbal/mnemo/releases). During the
`v1.0.0` release-candidate period, security fixes also target the latest
published `v1.0.0` prerelease. Older stable versions and prereleases are not
guaranteed backports.

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Prefer one of these private channels:

1. [GitHub Security Advisories](https://github.com/jmeiracorbal/mnemo/security/advisories/new)
   (recommended)
2. A private message to the repository maintainers on GitHub

Include:

- A clear description of the issue
- Steps to reproduce, or a proof of concept when safe
- Affected version / commit when known
- Impact assessment (data exposure, privilege escalation, remote code execution, etc.)

## What to expect

- Acknowledgement when the report is received
- An initial assessment and next steps
- Coordinated disclosure once a fix is available when that is appropriate

## Scope notes

mnemo runs locally, installs agent hooks/MCP configuration, and manages a
per-user SQLite store plus an event controller. Reports involving agent hook
injection, unauthorized store writes, installer integrity, or privilege
escalation through setup paths are in scope.

Issues that only affect third-party agent applications (Claude Code, Codex,
Cursor, OpenCode, Pi) should be reported upstream unless mnemo's integration is
the root cause.
