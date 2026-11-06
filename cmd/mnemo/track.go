package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

// runTrack explicitly opts an initialized Git project into tracking its .mnemo marker.
func runTrack() {
	dir := "."
	for _, arg := range os.Args[2:] {
		switch {
		case strings.HasPrefix(arg, "--path="):
			dir = arg[len("--path="):]
		default:
			fmt.Fprintf(os.Stderr, "mnemo track: unknown argument %q\n", arg)
			os.Exit(1)
		}
	}

	abs, err := absPath(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo track: %v\n", err)
		os.Exit(1)
	}
	root := agentinit.ProjectRoot(abs)
	id, err := agentinit.ReadProjectID(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo track: %v\n", err)
		os.Exit(1)
	}
	if err := agentinit.EnsureMarkerTracked(root); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo track: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("mnemo track: .mnemo staged for Git in %s (project %s)\n", root, id)
	fmt.Println("mnemo track: review the changes and commit them; mnemo does not commit or push.")
}
