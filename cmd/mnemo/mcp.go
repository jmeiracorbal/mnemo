package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jmeiracorbal/mnemo/internal/events"
	mcpserver "github.com/jmeiracorbal/mnemo/internal/mcp"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/server"
)

func runMCP(s *store.Store) {
	cfg, err := events.LoadConfig(s.DataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: event config: %v\n", err)
		os.Exit(1)
	}
	controller, err := events.EnsureController(context.Background(), cfg, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: event controller: %v\n", err)
		os.Exit(1)
	}
	if controller != nil {
		defer controller.Close()
	}
	tools := ""
	for _, arg := range os.Args[2:] {
		if len(arg) > 8 && arg[:8] == "--tools=" {
			tools = arg[8:]
		}
	}

	allowlist := mcpserver.ResolveTools(tools)
	srv, runtime, err := mcpserver.NewServerWithRuntime(s, version, allowlist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := runtime.Close(s); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: close MCP sessions: %v\n", err)
		}
	}()

	if err := server.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server error: %v\n", err)
		os.Exit(1)
	}
}
