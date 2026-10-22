# Releases and version metadata

Read this guide before changing version references, installer/update behavior,
or preparing a release.

## Version boundary

The binary version is injected with build flags. The MCP server must advertise
that same binary version and must not use an independent constant.

For a release, update the example pinned version in `install.sh` and the
README installation commands (English, Spanish and Chinese). Search for stale
references before tagging. The installation commands for a release must match
an actually published tag: do not advertise a new tag before its assets exist.
Unpinned installation follows the latest stable GitHub release.

## Post-merge release workflow

Only after an approved PR is merged into `main`:

1. Check out current `main` and fast-forward it with tags.
2. Update version metadata and documentation for the released behavior, but
   leave pre-release installation commands functional until the new tag exists.
3. Run verification and merge the release-ready changes into `main`.
4. Tag the exact verified `main` commit and publish its release assets only if
   the release workflow's reusable CI verification passes.
5. Once assets exist, update pinned examples and remove temporary beta-only
   wording from the English, Spanish and Chinese READMEs and installer comment.
6. Run the remote installer in an isolated `HOME` and `MNEMO_INSTALL_DIR`.
   Verify the downloaded binary version, warning-free global setup, fresh
   project init/doctor, and one affected command end to end.

Never tag from a feature branch or stale `main`; a local checkout build is not
a substitute for the isolated remote-install verification.
