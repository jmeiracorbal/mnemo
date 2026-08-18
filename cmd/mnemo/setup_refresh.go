package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	updated, err := refreshSetup(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup refresh: %v\n", err)
		os.Exit(1)
	}
	for _, path := range updated {
		fmt.Printf("mnemo setup refresh: updated %s\n", path)
	}
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

func refreshSetup(opts setupRefreshOptions) ([]string, error) {
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return nil, err
	}
	var updated []string
	for _, agent := range agents {
		agentUpdated, err := agentinit.Refresh(opts.Home, opts.MnemoBin, agent)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", agent, err)
		}
		updated = append(updated, agentUpdated...)
	}
	return updated, nil
}
