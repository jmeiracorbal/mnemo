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

func claudeNativeMemoryPath(home, root string) string {
	candidates := claudeProjectDirCandidates(home, root)
	for _, dir := range candidates {
		path := filepath.Join(dir, "memory", "MEMORY.md")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func claudeProjectDirCandidates(home, root string) []string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return nil
	}

	var encoded []string
	rel := absRoot
	if strings.HasPrefix(absRoot, home+string(os.PathSeparator)) {
		rel = strings.TrimPrefix(absRoot, home+string(os.PathSeparator))
	} else if absRoot == home {
		rel = ""
	} else if strings.HasPrefix(absRoot, "/") {
		rel = strings.TrimPrefix(absRoot, "/")
	}
	if rel != "" {
		encoded = append(encoded, strings.ToLower(strings.ReplaceAll(rel, string(os.PathSeparator), "-")))
	}
	encoded = append(encoded, strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(absRoot, "/"), string(os.PathSeparator), "-")))

	seen := make(map[string]struct{}, len(encoded))
	var dirs []string
	for _, name := range encoded {
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "-") {
			name = "-" + name
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		dirs = append(dirs, filepath.Join(home, ".claude", "projects", name))
	}
	return dirs
}
