// Package mcp implements the Model Context Protocol server for mnemo.
//
// This exposes memory tools via MCP stdio transport so any agent
// (Claude Code, OpenCode, Gemini CLI, Codex, etc.) can use mnemo's
// persistent memory just by adding it as an MCP server.
//
// Tool profiles allow agents to load only the tools they need:
//
//	mnemo mcp                    → all tools (default)
//	mnemo mcp --tools=agent      → 11 tools agents actually use
//	mnemo mcp --tools=admin      → 3 tools for CLI curation (delete, stats, timeline)
//	mnemo mcp --tools=mem_save,mem_search → individual tool names
package mcp

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo-adapters/agents"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Runtime owns one MCP stdio server process. Its per-process instance ID is
// distinct from an agent execution identity; an adapter may bind a session to
// a documented native execution ID carried by a tools/call request.
type Runtime struct {
	instanceID string
	pid        int
	agent      agents.Agent
	mu         sync.Mutex
}

// NewServer creates an MCP server with ALL tools registered.
func NewServer(s MemoryBackend, version string) (*server.MCPServer, error) {
	return NewServerWithTools(s, version, nil)
}

// NewServerWithTools creates an MCP server registering only the tools in
// the allowlist. If allowlist is nil, all tools are registered.
// version is the binary version advertised to MCP clients; it is required.
func NewServerWithTools(s MemoryBackend, version string, allowlist map[string]bool) (*server.MCPServer, error) {
	srv, _, err := NewServerWithRuntime(s, version, allowlist)
	return srv, err
}

// NewServerWithRuntime creates an MCP server and the runtime that owns its
// sessions. Call Close after the stdio transport terminates.
func NewServerWithRuntime(s MemoryBackend, version string, allowlist map[string]bool) (*server.MCPServer, *Runtime, error) {
	if strings.TrimSpace(version) == "" {
		return nil, nil, fmt.Errorf("mcp server version is required")
	}
	agent := agents.Agent(strings.TrimSpace(os.Getenv("MNEMO_AGENT")))
	if agent != "" {
		if _, ok := agents.AdapterFor(agent); !ok {
			return nil, nil, fmt.Errorf("unknown MCP agent %q", agent)
		}
	}
	runtime := &Runtime{instanceID: uuid.NewString(), pid: os.Getpid(), agent: agent}

	srv := server.NewMCPServer(
		"mnemo",
		version,
		server.WithToolCapabilities(true),
		server.WithInstructions(strings.TrimSpace(serverInstructions)),
	)

	registerTools(srv, s, allowlist, runtime)
	return srv, runtime, nil
}

// Close releases all open sessions owned by this runtime. It is safe to call
// after a read-only MCP connection that never created a session.
func (r *Runtime) Close(s MemoryBackend) error {
	return s.CloseMCPInstanceSessions(r.instanceID)
}

func (r *Runtime) resolveSession(s MemoryBackend, project, directory string, req mcp.CallToolRequest) (string, error) {
	identity, bindExecution, err := r.executionIdentity(project, req)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sessionID, err := s.ResolveMCPInstanceSession(project, directory, r.instanceID, r.pid)
	if err != nil {
		return "", err
	}
	if !bindExecution {
		return sessionID, nil
	}
	if err := s.BindExecutionSession(project, identity.Agent, identity.NativeID, sessionID); err != nil {
		return "", err
	}
	return sessionID, nil
}

func (r *Runtime) executionIdentity(project string, req mcp.CallToolRequest) (agents.Identity, bool, error) {
	if adapter, ok := agents.ToolCallAdapterFor(r.agent); ok {
		if req.Params.Meta == nil {
			return agents.Identity{}, true, fmt.Errorf("%s tools/call metadata is required", r.agent)
		}
		identity, err := adapter.ParseToolCallMeta(req.Params.Meta.AdditionalFields, project)
		if err != nil {
			return agents.Identity{}, true, err
		}
		return identity, true, nil
	}
	if adapter, ok := agents.EnvironmentAdapterFor(r.agent); ok {
		variable := adapter.NativeIDEnvironmentVariable()
		nativeID := strings.TrimSpace(os.Getenv(variable))
		if nativeID == "" {
			return agents.Identity{}, true, fmt.Errorf("%s requires %s", r.agent, variable)
		}
		identity, err := adapter.Identity(project, nativeID)
		if err != nil {
			return agents.Identity{}, true, err
		}
		return identity, true, nil
	}
	if adapter, ok := agents.SideChannelAdapterFor(r.agent); ok {
		nativeID, err := adapter.ReadNativeID(project)
		if err != nil {
			return agents.Identity{}, true, err
		}
		identity, err := adapter.Identity(project, nativeID)
		if err != nil {
			return agents.Identity{}, true, err
		}
		return identity, true, nil
	}
	return agents.Identity{}, false, nil
}
