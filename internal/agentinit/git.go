package agentinit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureMarkerIgnored excludes the project marker from Git without touching
// Git's index. A previously tracked marker requires explicit user intervention.
func EnsureMarkerIgnored(root string) error {
	if _, err := os.Lstat(filepath.Join(root, ".git")); errors.Is(err, os.ErrNotExist) {
		return nil // mnemo is usable outside Git.
	} else if err != nil {
		return fmt.Errorf("inspect Git worktree: %w", err)
	}

	check := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", "--", markerName)
	if out, err := check.CombinedOutput(); err == nil {
		return fmt.Errorf(".mnemo is already tracked; run git rm --cached -- .mnemo before initializing: %s", root)
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return fmt.Errorf("check marker tracking: %s: %w", strings.TrimSpace(string(out)), err)
	}

	path := filepath.Join(root, ".gitignore")
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read .gitignore: %w", err)
	}
	// Keep an explicit final rule: a preceding negation must not expose .mnemo.
	for i := len(strings.Split(string(content), "\n")) - 1; i >= 0; i-- {
		line := strings.TrimSpace(strings.Split(string(content), "\n")[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "/.mnemo" {
			return nil
		}
		break
	}
	addition := "/.mnemo\n"
	if len(content) > 0 && content[len(content)-1] != '\n' {
		addition = "\n" + addition
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	if _, err := file.WriteString(addition); err != nil {
		_ = file.Close()
		return fmt.Errorf("update .gitignore: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close .gitignore: %w", err)
	}
	return nil
}
