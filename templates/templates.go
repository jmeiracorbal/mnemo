package templates

import _ "embed"

//go:embed rules/generic.md
var Generic string

//go:embed rules/claudecode.md
var ClaudeCode string

//go:embed rules/cursor.mdc
var Cursor string

//go:embed rules/windsurf.md
var Windsurf string
