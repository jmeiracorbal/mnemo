package mcp

import (
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/v2/protocol"
)

// Provenance belongs to Mnemo's MCP adapter, not to the store or to a harness.
func mcpProvenance(tool string) protocol.Provenance {
	agent := strings.TrimSpace(os.Getenv("MNEMO_AGENT"))
	if agent == "" { agent = "unknown" }
	clientID := strings.TrimSpace(os.Getenv("MNEMO_MCP_CLIENT"))
	if clientID == "" { clientID = agent }
	return protocol.Provenance{
		AgentID: agent, SourceKindID: "mcp", ToolID: tool,
		ModelID: strings.TrimSpace(os.Getenv("MNEMO_MODEL")),
		ModelProvider: strings.TrimSpace(os.Getenv("MNEMO_MODEL_PROVIDER")),
		MCPClientID: clientID,
		MCPClientName: strings.TrimSpace(os.Getenv("MNEMO_MCP_CLIENT_NAME")),
		MCPClientVersion: strings.TrimSpace(os.Getenv("MNEMO_MCP_CLIENT_VERSION")),
		MCPTransport: "stdio",
	}
}
