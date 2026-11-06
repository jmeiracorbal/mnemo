package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmeiracorbal/mnemo-events"
	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupRefreshOptions struct {
	Agent    string
	Home     string
	MnemoBin string
}

func runSetupRefresh() {
	opts, err := parseSetupRefreshArgs(os.Args[3:], os.UserHomeDir, exec.LookPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup refresh: %v\n", err)
		os.Exit(1)
	}
	result, err := refreshSetup(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup refresh: %v\n", err)
		os.Exit(1)
	}
	for _, path := range result.Updated {
		fmt.Printf("mnemo setup refresh: updated %s\n", path)
	}
	if result.ControllerRestarted {
		fmt.Println("mnemo setup refresh: restarted controller service")
	}
	if len(result.Updated) == 0 && !result.ControllerRestarted {
		fmt.Println("mnemo setup refresh: already up to date")
	}
}

type setupRefreshResult struct {
	// Updated lists the managed paths created, modified or removed by this run.
	Updated             []string
	ControllerRestarted bool
}

func parseSetupRefreshArgs(args []string, userHomeDir func() (string, error), lookPath func(string) (string, error)) (setupRefreshOptions, error) {
	opts := setupRefreshOptions{Agent: "all"}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		case strings.HasPrefix(arg, "--mnemo-bin="):
			opts.MnemoBin = arg[len("--mnemo-bin="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.Agent == "" {
		opts.Agent = "all"
	}
	if opts.Home == "" {
		home, err := userHomeDir()
		if err != nil {
			return opts, fmt.Errorf("home: %w", err)
		}
		opts.Home = home
	}
	if opts.MnemoBin == "" {
		if path, err := lookPath("mnemo"); err == nil && strings.TrimSpace(path) != "" {
			opts.MnemoBin = path
		} else {
			opts.MnemoBin = "mnemo"
		}
	}
	return opts, nil
}

func refreshSetup(opts setupRefreshOptions) (setupRefreshResult, error) {
	var result setupRefreshResult
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return result, err
	}
	// Writers leave identical files untouched, so a path counts as updated
	// when this run modified or removed it.
	start := time.Now()
	var managed []string
	configPath, err := events.EnsureConfig(filepath.Join(opts.Home, ".mnemo"))
	if err != nil {
		return result, fmt.Errorf("event config: %w", err)
	}
	managed = append(managed, configPath)
	actualHome, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return result, fmt.Errorf("resolve service home: %w", homeErr)
	}
	if filepath.Clean(opts.Home) == filepath.Clean(actualHome) {
		servicePath, restarted, err := installControllerService(opts.Home, opts.MnemoBin)
		if err != nil {
			return result, fmt.Errorf("controller service: %w", err)
		}
		managed = append(managed, servicePath)
		result.ControllerRestarted = restarted
	}

	skillFiles, err := agentinit.InstallGlobalSkillForAgents(opts.Home, agents)
	if err != nil {
		return result, fmt.Errorf("global skill: %w", err)
	}
	managed = append(managed, skillFiles...)

	for _, agent := range agents {
		agentUpdated, err := agentinit.Refresh(opts.Home, opts.MnemoBin, agent)
		if err != nil {
			return result, fmt.Errorf("%s: %w", agent, err)
		}
		managed = append(managed, agentUpdated...)
	}
	result.Updated = changedSince(managed, start)
	return result, nil
}

// changedSince returns the unique paths modified at or after start, or no
// longer present.
func changedSince(paths []string, start time.Time) []string {
	seen := make(map[string]struct{}, len(paths))
	var changed []string
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		info, err := os.Lstat(path)
		if err == nil && info.ModTime().Before(start) {
			continue
		}
		changed = append(changed, path)
	}
	return changed
}
