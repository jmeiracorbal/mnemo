package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// validateMarker is the Capa-2 deterministic guardrail. It confirms that
// directory contains a .mnemo marker whose id matches project before any write
// reaches the controller. The git root of directory is resolved first so the
// check works regardless of which subdirectory the agent passed.
func validateMarker(project, directory string) error {
	if strings.TrimSpace(project) == "" {
		return fmt.Errorf("mnemo: project is required")
	}
	if strings.TrimSpace(directory) == "" {
		return fmt.Errorf("mnemo: directory is required")
	}
	root := markerGitRoot(directory)
	data, err := os.ReadFile(filepath.Join(root, ".mnemo"))
	if err != nil {
		return fmt.Errorf("mnemo: project not initialized at %s — run 'mnemo init' to activate persistent memory", root)
	}
	var marker struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &marker); err != nil || strings.TrimSpace(marker.ID) == "" {
		return fmt.Errorf("mnemo: invalid .mnemo marker at %s — run 'mnemo init'", root)
	}
	if marker.ID != project {
		return fmt.Errorf("mnemo: project id mismatch — .mnemo has %q but request carries %q", marker.ID, project)
	}
	return nil
}

func markerGitRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	return strings.TrimSpace(string(out))
}
