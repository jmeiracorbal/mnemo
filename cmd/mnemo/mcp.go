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

func runMCP() {
	// MCP deliberately loads only technical configuration. The global
	// controller owns Store and is the sole SQLite process.
	storeConfig, err := store.DefaultConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: config error: %v\n", err)
		os.Exit(1)
	}
	cfg, err := events.LoadConfig(storeConfig.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: event config: %v\n", err)
		os.Exit(1)
	}
	if err := events.CheckHealth(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: global controller unavailable: %v\n", err)
		os.Exit(1)
	}
	tools := ""
	for _, arg := range os.Args[2:] {
		if len(arg) > 8 && arg[:8] == "--tools=" {
			tools = arg[8:]
		}
	}

	allowlist := mcpserver.ResolveTools(tools)
	backend := mcpserver.NewControllerBackend(cfg)
	srv, runtime, err := mcpserver.NewServerWithRuntime(backend, version, allowlist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := runtime.Close(backend); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: close MCP sessions: %v\n", err)
		}
	}()

	if err := server.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server error: %v\n", err)
		os.Exit(1)
	}
}
