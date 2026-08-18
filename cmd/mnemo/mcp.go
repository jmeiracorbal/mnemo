package main

import (
	"fmt"
	"os"

	mcpserver "github.com/jmeiracorbal/mnemo/internal/mcp"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/server"
)

func runMCP(s *store.Store) {
	tools := ""
	for _, arg := range os.Args[2:] {
		if len(arg) > 8 && arg[:8] == "--tools=" {
			tools = arg[8:]
		}
	}

	allowlist := mcpserver.ResolveTools(tools)
	srv, err := mcpserver.NewServerWithTools(s, version, allowlist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server: %v\n", err)
		os.Exit(1)
	}

	if err := server.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server error: %v\n", err)
		os.Exit(1)
	}
}
