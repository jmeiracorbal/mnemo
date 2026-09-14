# Releases and version metadata

Read this guide before changing version references, installer/update behavior,
plugin metadata, or preparing a release.

## Version boundary

The binary version is injected with build flags. The MCP server must advertise
that same binary version and must not use an independent constant.

Every version bump updates both:

- `.claude-plugin/marketplace.json` — `plugins[0].version`
- `plugin/claude-code/.claude-plugin/plugin.json` — `version`

Search for the previous version, update all required references, and confirm
there are no stale values before committing or tagging. The release tag and
plugin metadata version must match.

The root `.mcp.json` is for development in this repository; the copy under
`plugin/claude-code/.mcp.json` is for installed users. Update both whenever the
MCP configuration changes.

## Post-merge release workflow

Only after an approved PR is merged and published to `main`:

1. Check out current `main` and fast-forward it with tags.
2. Update version metadata and documentation for the released behavior.
3. Run verification, commit, tag the matching version, and push commit and tag.
4. Run the remote installer in an isolated `HOME` and `MNEMO_INSTALL_DIR`.
   Verify the downloaded binary version, warning-free global setup, fresh
   project init/doctor, and one affected command end to end.

Never tag from a feature branch or stale `main`; a local checkout build is not
a substitute for the isolated remote-install verification.
