package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/jmeiracorbal/mnemo/protocol"
	gomcp "github.com/mark3labs/mcp-go/mcp"
)

type recordingProtocolBackend struct {
	MemoryBackend
	sessionID string
	created   protocol.ObservationCreateInput
	getRef    protocol.ResourceRef
}

func (b *recordingProtocolBackend) ResolveSession(_ context.Context, _ string, _ protocol.SessionResolveInput) (string, error) {
	return b.sessionID, nil
}

func (b *recordingProtocolBackend) CreateObservation(_ context.Context, id string, input protocol.ObservationCreateInput) (protocol.ResourceRef, error) {
	if strings.TrimSpace(id) == "" {
		return protocol.ResourceRef{}, &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "request ID missing"}
	}
	b.created = input
	return protocol.NewObservationRef(input.ProjectID, "obs-recorder")
}

func (b *recordingProtocolBackend) GetObservationRef(_ context.Context, ref protocol.ResourceRef) (protocol.Observation, error) {
	b.getRef = ref
	return protocol.Observation{
		Ref: ref, SessionID: "session-1", Type: "manual", Title: "Title",
		Content: "Complete content", Scope: "project",
	}, nil
}

func (*recordingProtocolBackend) MaxObservationLength() int { return 10000 }

func (b *recordingProtocolBackend) Search(string, store.SearchOptions) ([]store.SearchResult, error) {
	project := "project-1"
	return []store.SearchResult{{Observation: store.Observation{
		ID: 4, SyncID: "obs-record", Project: &project, Type: "manual",
		Title: "Title", Content: "Complete content",
	}}}, nil
}

func makeProtocolToolRequest(t *testing.T, name string, args map[string]any) gomcp.CallToolRequest {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"params": map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	var request gomcp.CallToolRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	return request
}

func TestMCPSaveUsesPublicContractWithActualAgentSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mnemo"), []byte(`{"id":"project-1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	backend := &recordingProtocolBackend{sessionID: "mcp-session-1"}
	runtime := &Runtime{instanceID: "test-mcp", pid: 1}
	request := makeProtocolToolRequest(t, "mem_save", map[string]any{
		"title": "Decision", "content": "Persisted decision",
		"project": "project-1", "directory": dir, "type": "decision",
	})
	result, err := handleSave(backend, runtime)(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.IsError {
		t.Fatalf("MCP save failed: %+v", result)
	}
	input := backend.created
	if input.SessionID != "mcp-session-1" || input.ProjectID != "project-1" ||
		input.Type != "decision" || input.Title != "Decision" {
		t.Fatalf("MCP lost session or observation fields: %+v", input)
	}
	if input.Provenance.ToolID == "" {
		t.Fatalf("MCP lost memory provenance: %+v", input.Provenance)
	}
}

func TestMCPGetObservationRequiresCanonicalRef(t *testing.T) {
	backend := &recordingProtocolBackend{}
	raw := "mnemo://project-1/observations/obs-record"
	request := makeProtocolToolRequest(t, "mem_get_observation", map[string]any{"ref": raw})
	result, err := handleGetObservation(backend)(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.IsError || backend.getRef.String() != raw {
		t.Fatalf("get did not use canonical Mnemo ref: %+v, ref=%s", result, backend.getRef.String())
	}

	legacy := makeProtocolToolRequest(t, "mem_get_observation", map[string]any{"id": 123})
	result, err = handleGetObservation(backend)(context.Background(), legacy)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.IsError {
		t.Fatal("legacy numeric ID accepted as a ref")
	}
}

func TestMCPSearchProvidesStableRef(t *testing.T) {
	backend := &recordingProtocolBackend{}
	request := makeProtocolToolRequest(t, "mem_search", map[string]any{"query": "Title"})
	result, err := handleSearch(backend)(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.IsError {
		t.Fatalf("MCP search failed: %+v", result)
	}
	text := ""
	for _, item := range result.Content {
		if content, ok := item.(gomcp.TextContent); ok {
			text += content.Text
		}
	}
	if !strings.Contains(text, "mnemo://project-1/observations/obs-record") {
		t.Fatalf("MCP search omitted canonical reference: %s", text)
	}
}
