package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	"github.com/jmeiracorbal/mnemo/protocol"
)

func TestPublicArchiveImportIsAtomicAndIdempotent(t *testing.T) {
	src, err := New(FallbackConfig(t.TempDir()))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = src.Close() })
	sessionID, err := src.ResolveMCPInstanceSession("import-project", t.TempDir(), "instance-import", 1)
	if err != nil { t.Fatal(err) }
	if _, err := src.AddObservation(AddObservationParams{
		SessionID: sessionID, Project: "import-project", Type: "manual",
		Title: "imported obs", Content: "content for import test",
	}); err != nil { t.Fatal(err) }
	exported, err := src.ExecuteProtocolRequest(context.Background(), mustPublicRequest(t, protocol.OperationArchiveExport, "", nil))
	if err != nil { t.Fatal(err) }
	var exportedResponse protocol.Response
	if err := json.Unmarshal(exported, &exportedResponse); err != nil { t.Fatal(err) }
	if exportedResponse.Error != nil { t.Fatal(exportedResponse.Error) }
	var archive protocol.ArchiveExportOutput
	if err := json.Unmarshal(exportedResponse.Payload, &archive); err != nil { t.Fatal(err) }
	dst, err := New(FallbackConfig(t.TempDir()))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = dst.Close() })
	request := mustPublicRequest(t, protocol.OperationArchiveImport, "import-cmd-1", protocol.ArchiveImportInput{Archive: archive})
	first, err := dst.ApplyCommand("transport-1", request)
	if err != nil { t.Fatal(err) }
	var firstResponse protocol.Response
	if err := json.Unmarshal(first, &firstResponse); err != nil { t.Fatal(err) }
	if firstResponse.Error != nil { t.Fatal(firstResponse.Error) }
	var imported protocol.ArchiveImportOutput
	if err := json.Unmarshal(firstResponse.Payload, &imported); err != nil { t.Fatal(err) }
	if imported.ObservationsImported != 1 { t.Fatalf("imported %d observations", imported.ObservationsImported) }
	second, err := dst.ApplyCommand("transport-2", request)
	if err != nil { t.Fatal(err) }
	if string(first) != string(second) { t.Fatal("replayed import returned a different result") }
	rows, err := dst.q.ExportObservations(context.Background())
	if err != nil { t.Fatal(err) }
	if len(rows) != 1 || rows[0].Title != "imported obs" {
		t.Fatalf("import was not atomic or duplicated: %+v", rows)
	}
}

func mustPublicRequest(t *testing.T, operation protocol.Operation, id string, payload any) json.RawMessage {
	t.Helper()
	request, err := protocol.NewRequest(operation, id, payload)
	if err != nil { t.Fatal(err) }
	body, err := json.Marshal(request)
	if err != nil { t.Fatal(err) }
	return body
}

func TestDurableCommandReturnsStoredResultWithoutRepeatingMutation(t *testing.T) {
	memory, err := New(FallbackConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	sessionID, err := memory.ResolveMCPInstanceSession("project-command", t.TempDir(), "instance-command", 1)
	if err != nil {
		t.Fatal(err)
	}
	request, err := protocol.NewRequest(protocol.OperationObservationCreate, "command-1", protocol.ObservationCreateInput{
		SessionID: sessionID, Type: "manual", Title: "exactly once",
		Content: "a durable command must not repeat", ProjectID: "project-command",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	first, err := memory.ApplyCommand("delivery-1", payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := memory.ApplyCommand("delivery-2", payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("replayed result = %s, want %s", second, first)
	}
	observations, err := memory.SessionObservations(sessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 {
		t.Fatalf("observations = %d, want 1", len(observations))
	}
}

func TestNativeSessionResolveUsesCanonicalExecutionIdentity(t *testing.T) {
	memory, err := New(FallbackConfig(t.TempDir()))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = memory.Close() })
	const project = "project-pi-tool"
	const nativeID = "pi-native-session"
	input := protocol.SessionResolveInput{
		ProjectID: project, AgentID: string(agents.AgentPi),
		NativeID: nativeID, Directory: t.TempDir(),
	}
	request, err := protocol.NewRequest(protocol.OperationSessionResolve, "pi-resolve-1", input)
	if err != nil { t.Fatal(err) }
	body, err := json.Marshal(request)
	if err != nil { t.Fatal(err) }
	raw, err := memory.ApplyCommand("delivery-1", body)
	if err != nil { t.Fatal(err) }
	var response protocol.Response
	if err := json.Unmarshal(raw, &response); err != nil { t.Fatal(err) }
	if response.Error != nil { t.Fatal(response.Error) }
	var out protocol.SessionResolveOutput
	if err := json.Unmarshal(response.Payload, &out); err != nil { t.Fatal(err) }
	identity, err := agents.NewIdentity(agents.AgentPi, project, nativeID)
	if err != nil { t.Fatal(err) }
	if out.SessionID != "execution-"+identity.Execution {
		t.Fatalf("native session = %q; want canonical execution session", out.SessionID)
	}
}

func TestRetiredActionStringIsNotPublicProtocol(t *testing.T) {
	memory, err := New(FallbackConfig(t.TempDir()))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = memory.Close() })
	for _, operation := range []protocol.Operation{"add_observation", "get_observation", "agent_tool", "search", "merge_projects"} {
		request := protocol.Request{Version: protocol.Version, Operation: operation, RequestID: "old-command", Payload: json.RawMessage(`{}`)}
		payload, err := json.Marshal(request)
		if err != nil { t.Fatal(err) }
		body, err := memory.ApplyCommand("old-command", payload)
		if err != nil { t.Fatal(err) }
		var response protocol.Response
		if err := json.Unmarshal(body, &response); err != nil { t.Fatal(err) }
		if response.Error == nil || response.Error.Code != protocol.CodeInvalidRequest {
			t.Fatalf("retired operation %q accepted: %s", operation, body)
		}
	}
}
