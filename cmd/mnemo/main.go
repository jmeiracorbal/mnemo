package main

import (
	"fmt"
	"os"
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
