package main

import (
	"fmt"
	"os"

	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-claude"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-codex"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-cursor"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-opencode"
	_ "github.com/jmeiracorbal/mnemo/internal/adapters/agents/mnemo-pi"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		maybeWarnUpdate(os.Args[1:])
	}

	root := newRootCommand()
	root.SetArgs(os.Args[1:])
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: %v\n", err)
		os.Exit(1)
	}
}
