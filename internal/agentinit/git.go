package agentinit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureMarkerTracked makes .mnemo versionable in an existing Git worktree.
// It removes only the legacy mnemo ignore entry and stages .mnemo if untracked.
// It never creates a Git repository, stages unrelated files, or changes the ID.
func EnsureMarkerTracked(root string) error {
	if _, err := os.Lstat(filepath.Join(root, ".git")); errors.Is(err, os.ErrNotExist) {
		// ProjectRoot passes a repository root when Git is available.
		// For projects outside Git, no Git-specific setup is necessary.
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect Git repository: %w", err)
	}

	cmd := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify Git worktree: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("not inside a Git worktree: %s", root)
	}
	if _, err := os.Lstat(filepath.Join(root, markerName)); err != nil {
		return fmt.Errorf("inspect .mnemo marker: %w", err)
	}

	if err := removeMarkerIgnoreRule(filepath.Join(root, ".gitignore")); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	trackedCmd := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", "--", markerName)
	if out, err := trackedCmd.CombinedOutput(); err == nil {
		return nil
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return fmt.Errorf("check .mnemo tracking: %s: %w", strings.TrimSpace(string(out)), err)
	}

	ignoredCmd := exec.Command("git", "-C", root, "check-ignore", "-q", "--", markerName)
	if out, err := ignoredCmd.CombinedOutput(); err == nil {
		source, _ := exec.Command("git", "-C", root, "check-ignore", "-v", "--", markerName).CombinedOutput()
		return fmt.Errorf(".mnemo is still excluded by Git (%s); update that ignore rule before tracking", strings.TrimSpace(string(source)))
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return fmt.Errorf("check .mnemo ignore rules: %s: %w", strings.TrimSpace(string(out)), err)
	}

	addCmd := exec.Command("git", "-C", root, "add", "--", markerName)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stage .mnemo: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// removeMarkerIgnoreRule removes only direct exclusions of the root .mnemo file.
// It preserves every other Git ignore rule and does not create .gitignore.
func removeMarkerIgnoreRule(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var updated strings.Builder
	changed := false
	for _, line := range strings.SplitAfter(string(data), "\n") {
		rule := strings.TrimSpace(line)
		if rule == ".mnemo" || rule == "/.mnemo" {
			changed = true
			continue
		}
		updated.WriteString(line)
	}
	if !changed {
		return nil
	}
	return os.WriteFile(path, []byte(updated.String()), 0644)
}
