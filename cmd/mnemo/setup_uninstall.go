package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupUninstallOptions struct {
	Agent string
	Home  string
}

func runSetupUninstall() {
	opts, err := parseSetupUninstallArgs(os.Args[3:], os.UserHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup uninstall: %v\n", err)
		os.Exit(1)
	}
	removed, err := uninstallSetup(opts)
	if err != nil {
		printSetupUninstallRemoved(removed)
		fmt.Fprintf(os.Stderr, "mnemo setup uninstall: %v\n", err)
		os.Exit(1)
	}
	printSetupUninstallRemoved(removed)
}

func printSetupUninstallRemoved(removed []string) {
	for _, path := range removed {
		fmt.Printf("mnemo setup uninstall: updated %s\n", path)
	}
}

func parseSetupUninstallArgs(args []string, userHomeDir func() (string, error)) (setupUninstallOptions, error) {
	var opts setupUninstallOptions
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.Agent == "" {
		return opts, fmt.Errorf("missing --agent=AGENT")
	}
	if opts.Home == "" {
		home, err := userHomeDir()
		if err != nil {
			return opts, fmt.Errorf("home: %w", err)
		}
		opts.Home = home
	}
	return opts, nil
}

func uninstallSetup(opts setupUninstallOptions) ([]string, error) {
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, agent := range agents {
		agentRemoved, err := agentinit.Uninstall(opts.Home, agent)
		removed = append(removed, agentRemoved...)
		if err != nil {
			return removed, fmt.Errorf("%s: %w", agent, err)
		}
	}
	return removed, nil
}
