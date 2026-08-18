package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupPrintConfigOptions struct {
	Agent    string
	Home     string
	MnemoBin string
}

func runSetupPrintConfig() {
	opts, err := parseSetupPrintConfigArgs(os.Args[3:], os.UserHomeDir, exec.LookPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup print-config: %v\n", err)
		os.Exit(1)
	}
	snippets, err := buildSetupConfigSnippets(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup print-config: %v\n", err)
		os.Exit(1)
	}
	printSetupConfigSnippets(snippets)
}

func parseSetupPrintConfigArgs(args []string, userHomeDir func() (string, error), lookPath func(string) (string, error)) (setupPrintConfigOptions, error) {
	opts := setupPrintConfigOptions{}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		case strings.HasPrefix(arg, "--mnemo-bin="):
			opts.MnemoBin = arg[len("--mnemo-bin="):]
		case strings.HasPrefix(arg, "--"):
			return opts, fmt.Errorf("unknown option %q", arg)
		case opts.Agent == "":
			opts.Agent = strings.TrimSpace(arg)
		default:
			return opts, fmt.Errorf("unexpected argument %q", arg)
		}
	}
	if opts.Agent == "" {
		return opts, fmt.Errorf("missing agent — usage: mnemo setup print-config AGENT")
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

func buildSetupConfigSnippets(opts setupPrintConfigOptions) ([]agentinit.ConfigSnippet, error) {
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return nil, err
	}
	var snippets []agentinit.ConfigSnippet
	for _, agent := range agents {
		agentSnippets, err := agentinit.ConfigSnippets(opts.Home, opts.MnemoBin, agent)
		if err != nil {
			return nil, err
		}
		snippets = append(snippets, agentSnippets...)
	}
	return snippets, nil
}

func printSetupConfigSnippets(snippets []agentinit.ConfigSnippet) {
	for i, snippet := range snippets {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("# %s: %s (%s)\n", snippet.Agent, snippet.Path, snippet.Format)
		fmt.Print(snippet.Content)
	}
}
