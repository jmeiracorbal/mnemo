package agentinit

import (
	"os"
	"path/filepath"
	"strings"
)

var competingMemoryFilenames = []string{
	"MEMORY.md",
	"memory.md",
}

// CheckCompetingMemory reports native or plaintext memory surfaces that conflict
// with mnemo when a valid .mnemo marker exists.
func CheckCompetingMemory(root, home string) Check {
	if _, err := ReadProjectID(root); err != nil {
		return Check{}
	}

	var conflicts []string
	for _, name := range competingMemoryFilenames {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
			conflicts = append(conflicts, path)
		}
	}

	if home != "" {
		if path := claudeNativeMemoryPath(home, root); path != "" {
			if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
				conflicts = append(conflicts, path)
			}
		}
	}

	if len(conflicts) == 0 {
		return checkOK("", "competing_memory", "no competing native memory surfaces detected", root)
	}

	return checkWarning(
		"",
		"competing_memory",
		"mnemo is authoritative for this project; do not write to native or plaintext memory surfaces: "+strings.Join(conflicts, ", "),
		root,
	)
}
