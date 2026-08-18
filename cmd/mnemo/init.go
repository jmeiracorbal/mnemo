package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

// runInit activates mnemo for one or more agents in the current project.
func runInit(s *store.Store) {
	agent := "claudecode"
	dir := "."
	projectRules := true

	for _, arg := range os.Args[2:] {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			agent = arg[len("--agent="):]
		case strings.HasPrefix(arg, "--path="):
			dir = arg[len("--path="):]
		case arg == "--no-project-rules":
			projectRules = false
		}
	}

	abs, err := absPath(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo init: %v\n", err)
		os.Exit(1)
	}
	root := agentinit.ProjectRoot(abs)

	projectID, err := agentinit.EnsureProjectID(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo init: project ID: %v\n", err)
		os.Exit(1)
	}
	name := filepath.Base(root)
	if err := s.EnsureProject(projectID, name); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo init: register project: %v\n", err)
		os.Exit(1)
	}

	agents, err := agentinit.ExpandAgents(agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo init: %v\n", err)
		os.Exit(1)
	}

	for _, a := range agents {
		if err := agentinit.AddAgent(root, a); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo init: %s: %v\n", a, err)
			os.Exit(1)
		}
		if projectRules {
			paths, err := agentinit.InstallProjectInstructions(root, a)
			if err != nil {
				fmt.Fprintf(os.Stderr, "mnemo init: %s project rules: %v\n", a, err)
				os.Exit(1)
			}
			for _, path := range paths {
				fmt.Printf("mnemo init: project rules updated %s\n", path)
			}
		}
		fmt.Printf("mnemo init: %s activated in %s\n", a, root)
	}
	if err := agentinit.EnsureGitignore(root); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo init: gitignore: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("mnemo init: project ID %s\n", projectID)
	if home, err := os.UserHomeDir(); err == nil {
		printSkillRecommendation(os.Stdout, home)
	}
}

func printSkillRecommendation(w io.Writer, home string) {
	if agentinit.GlobalSkillInstalled(home) {
		return
	}
	_, _ = fmt.Fprintln(w, "mnemo init: for a better agent experience, install the global mnemo-memory skill:")
	_, _ = fmt.Fprintln(w, "  npx skills add jmeiracorbal/mnemo --skill mnemo-memory --global")
}

// runMigrateProjects migrates a project from a legacy path-derived key to a
// UUID-based ID. Run once per project after upgrading to project-ID-based tracking.
//
// Steps:
//  1. Resolve the current project root.
//  2. Compute the deterministic UUID v5 from the absolute path.
//  3. Compute what the legacy derived key would have been for this path.
//  4. Rekey existing observations/sessions from legacy key → UUID.
//  5. Register the project in the projects table.
//  6. Write the UUID to .mnemo.
func runMigrateProjects(s *store.Store) {
	dir := "."
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--path=") {
			dir = arg[len("--path="):]
		}
	}

	abs, err := absPath(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo migrate: %v\n", err)
		os.Exit(1)
	}
	root := agentinit.ProjectRoot(abs)

	legacyRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		legacyRoot = resolved
	}

	projectID := agentinit.ProjectUUIDFromPath(root)
	legacyKey := deriveLegacyKey(legacyRoot)
	name := filepath.Base(root)

	result, err := s.MigrateProject(legacyKey, projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo migrate: rekey: %v\n", err)
		os.Exit(1)
	}

	if err := s.EnsureProject(projectID, name); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo migrate: register: %v\n", err)
		os.Exit(1)
	}

	if err := agentinit.EnsureMarkerWithID(root, projectID); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo migrate: write marker: %v\n", err)
		os.Exit(1)
	}

	if err := agentinit.EnsureGitignore(root); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo migrate: gitignore: %v\n", err)
		os.Exit(1)
	}

	if result.Migrated {
		fmt.Printf("migrated: %s -> %s (%d obs, %d sessions)\n",
			legacyKey, projectID, result.ObservationsUpdated, result.SessionsUpdated)
	} else {
		fmt.Printf("no legacy data found for %s, project registered as %s\n", legacyKey, projectID)
	}
}

// deriveLegacyKey computes what the path-derived project key would have been
// for the given absolute path under the old hook derivation logic:
//
//	realpath | sed "s|^$HOME/||; s|^/||" | tr '/' '-' | tr '[:upper:]' '[:lower:]'
func deriveLegacyKey(absPath string) string {
	home, _ := os.UserHomeDir()
	rel := absPath
	if home != "" && strings.HasPrefix(rel, home+"/") {
		rel = rel[len(home)+1:]
	} else if strings.HasPrefix(rel, "/") {
		rel = rel[1:]
	}
	return strings.ToLower(strings.ReplaceAll(rel, "/", "-"))
}

func absPath(dir string) (string, error) {
	return filepath.Abs(dir)
}
