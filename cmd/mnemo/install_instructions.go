package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

func runInstallInstructions() {
	agent, home, err := parseInstallInstructionsArgs(os.Args[2:], os.UserHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo install-instructions: home: %v\n", err)
		os.Exit(1)
	}

	agents, err := agentinit.ExpandAgents(agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo install-instructions: %v\n", err)
		os.Exit(1)
	}
	for _, a := range agents {
		path, err := agentinit.InstallGlobalInstructions(home, a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo install-instructions: %s: %v\n", a, err)
			os.Exit(1)
		}
		fmt.Printf("mnemo install-instructions: %s updated %s\n", a, path)
	}
}

func parseInstallInstructionsArgs(args []string, userHomeDir func() (string, error)) (agent, home string, err error) {
	agent = "all"
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			agent = arg[len("--agent="):]
		case strings.HasPrefix(arg, "--home="):
			home = arg[len("--home="):]
		}
	}
	if home != "" {
		return agent, home, nil
	}
	home, err = userHomeDir()
	return agent, home, err
}
