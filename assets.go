package mnemo

import "embed"

// SetupAssets contains the files that `mnemo setup refresh` can reinstall
// from the compiled binary.
//
//go:embed scripts/claudecode/hooks/* scripts/codex/hooks/* scripts/cursor/hooks/* scripts/opencode/plugins/* scripts/pi/extensions/* assets/protocol/* skills/mnemo-memory/** skills/mnemo-doctor/**
var SetupAssets embed.FS
