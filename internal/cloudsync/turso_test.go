package cloudsync

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc is an http.RoundTripper backed by a plain function,
// used to fake HTTP responses in tests without a real network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newTursoTestBackend creates a TursoBackend pointed at a fake URL with the given transport.
func newTursoTestBackend(t *testing.T, transport http.RoundTripper) *TursoBackend {
	t.Helper()
	b, err := NewTursoBackend(Config{
		URL:      "libsql://example.turso.io",
		Key:      "test-token",
		ClientID: "client-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	b.client.Transport = transport
	return b
}

// hranaOKBody builds a Hrana v2 success response for stmtCount statement results
// plus one close result at the end.
func hranaOKBody(stmtCount int) []byte {
	results := make([]map[string]any, stmtCount+1)
	for i := 0; i < stmtCount; i++ {
		results[i] = map[string]any{
			"type": "ok",
			"response": map[string]any{
				"type":   "execute",
				"result": map[string]any{"cols": []any{}, "rows": []any{}, "rows_affected": 0},
			},
		}
	}
	results[stmtCount] = map[string]any{"type": "ok", "response": map[string]any{"type": "close"}}
	body, _ := json.Marshal(map[string]any{"results": results})
	return body
}

// hranaRowsBody builds a Hrana v2 success response for a single SELECT statement
// that returns the given rows, plus one close result.
func hranaRowsBody(rows [][]hranaValue) []byte {
	type jVal = map[string]any
	var jRows [][]jVal
	for _, row := range rows {
		jRow := make([]jVal, len(row))
		for i, v := range row {
			jRow[i] = jVal{"type": v.Type, "value": v.Value}
		}
		jRows = append(jRows, jRow)
	}
	if jRows == nil {
		jRows = [][]jVal{}
	}
	results := []map[string]any{
		{
			"type": "ok",
			"response": map[string]any{
				"type":   "execute",
				"result": map[string]any{"cols": []any{}, "rows": jRows, "rows_affected": 0},
			},
		},
		{"type": "ok", "response": map[string]any{"type": "close"}},
	}
	body, _ := json.Marshal(map[string]any{"results": results})
	return body
}

func TestTursoBackendPushEmptyEntriesReturnsEmptyResult(t *testing.T) {
	httpCalled := false
	backend := newTursoTestBackend(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		httpCalled = true
		return nil, nil
	}))
	res, err := backend.PushMutations(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AcceptedSeqs) != 0 {
		t.Fatalf("expected empty AcceptedSeqs, got %v", res.AcceptedSeqs)
	}
	if httpCalled {
		t.Fatal("expected no HTTP call for empty entries")
	}
}

func TestTursoBackendPushSetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaOKBody(4))),
			Header:     make(http.Header),
		}, nil
	}))
	_, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 1, Entity: "session", EntityKey: "s1", Op: "upsert",
			Payload: json.RawMessage(`{"id":"s1","project":"p","directory":"/d"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected 'Bearer test-token', got %q", gotAuth)
	}
}

func TestTursoBackendPushSessionMutationWrapsInTransaction(t *testing.T) {
	var bodyStr string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodyStr = string(b)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaOKBody(4))),
			Header:     make(http.Header),
		}, nil
	}))
	res, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 7, Entity: "session", EntityKey: "s1", Op: "upsert",
			Payload: json.RawMessage(`{"id":"s1","project":"brain","directory":"/home"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AcceptedSeqs) != 1 || res.AcceptedSeqs[0] != 7 {
		t.Fatalf("expected AcceptedSeqs=[7], got %v", res.AcceptedSeqs)
	}
	for _, want := range []string{"BEGIN", "INSERT OR REPLACE INTO sessions", "COMMIT"} {
		if !strings.Contains(bodyStr, want) {
			t.Fatalf("expected %q in pipeline body", want)
		}
	}
}

func TestTursoBackendPushJournalRowUsesInsertOrIgnore(t *testing.T) {
	var bodyStr string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodyStr = string(b)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaOKBody(4))),
			Header:     make(http.Header),
		}, nil
	}))
	_, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 1, Entity: "session", EntityKey: "s1", Op: "upsert",
			Payload: json.RawMessage(`{"id":"s1","project":"p","directory":"/d"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bodyStr, "INSERT OR IGNORE INTO sync_mutations") {
		t.Fatalf("expected INSERT OR IGNORE journal write, body=%s", bodyStr)
	}
}

func TestTursoBackendPushObservationMutationUsesInsertOrReplace(t *testing.T) {
	var bodyStr string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodyStr = string(b)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaOKBody(4))),
			Header:     make(http.Header),
		}, nil
	}))
	_, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 2, Entity: "observation", EntityKey: "obs-1", Op: "upsert",
			Payload: json.RawMessage(`{"sync_id":"obs-1","session_id":"sess-1","type":"decision","title":"T","content":"C","scope":"project"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bodyStr, "INSERT OR REPLACE INTO observations") {
		t.Fatalf("expected INSERT OR REPLACE INTO observations, body=%s", bodyStr)
	}
}

func TestTursoBackendPushObservationDeleteMutationUsesUpdate(t *testing.T) {
	var bodyStr string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodyStr = string(b)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaOKBody(4))),
			Header:     make(http.Header),
		}, nil
	}))
	_, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 3, Entity: "observation", EntityKey: "obs-del", Op: "delete",
			OccurredAt: "2026-09-01T10:00:00Z",
			Payload:    json.RawMessage(`{"sync_id":"obs-del","session_id":"sess-1","type":"decision","title":"T","content":"C","scope":"project"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bodyStr, "INSERT OR REPLACE INTO observations") {
		t.Fatal("delete op must not use INSERT OR REPLACE INTO observations")
	}
	if !strings.Contains(bodyStr, "UPDATE observations SET deleted_at") {
		t.Fatalf("expected UPDATE observations SET deleted_at, body=%s", bodyStr)
	}
}

func TestTursoBackendPushPromptMutationUsesInsertOrReplace(t *testing.T) {
	var bodyStr string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodyStr = string(b)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaOKBody(4))),
			Header:     make(http.Header),
		}, nil
	}))
	_, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 4, Entity: "prompt", EntityKey: "pr-1", Op: "upsert",
			Payload: json.RawMessage(`{"sync_id":"pr-1","session_id":"sess-1","content":"Hello world"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bodyStr, "INSERT OR REPLACE INTO user_prompts") {
		t.Fatalf("expected INSERT OR REPLACE INTO user_prompts, body=%s", bodyStr)
	}
}

func TestTursoBackendPullBuildsCursorQueryWithClientExclusion(t *testing.T) {
	var capturedSQL string
	backend := newTursoTestBackend(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// Parse the Hrana pipeline request to extract decoded SQL (avoids JSON > escaping).
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Requests []struct {
				Stmt *struct {
					SQL string `json:"sql"`
				} `json:"stmt"`
			} `json:"requests"`
		}
		if err := json.Unmarshal(b, &req); err == nil {
			for _, rq := range req.Requests {
				if rq.Stmt != nil && strings.Contains(rq.Stmt.SQL, "SELECT") {
					capturedSQL = rq.Stmt.SQL
				}
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaRowsBody(nil))),
			Header:     make(http.Header),
		}, nil
	}))
	res, err := backend.PullMutations(10, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Mutations) != 0 {
		t.Fatalf("expected 0 mutations, got %d", len(res.Mutations))
	}
	for _, want := range []string{"seq > ?", "origin_id != ?"} {
		if !strings.Contains(capturedSQL, want) {
			t.Fatalf("expected %q in pull SQL, got: %s", want, capturedSQL)
		}
	}
}

func TestTursoBackendPullReturnsRowsFromResponse(t *testing.T) {
	rows := [][]hranaValue{
		{
			intVal(12), textVal("client-b"), intVal(3), textVal("brain"),
			textVal("session"), textVal("s1"), textVal("upsert"),
			textVal(`{"id":"s1","project":"brain","directory":"/home"}`),
			textVal("2026-09-01T10:00:00Z"),
		},
	}
	backend := newTursoTestBackend(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(hranaRowsBody(rows))),
			Header:     make(http.Header),
		}, nil
	}))
	res, err := backend.PullMutations(5, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Mutations) != 1 {
		t.Fatalf("expected 1 mutation, got %d", len(res.Mutations))
	}
	m := res.Mutations[0]
	if m.Seq != 12 || m.OriginID != "client-b" || m.Project != "brain" || m.Entity != "session" {
		t.Fatalf("unexpected mutation: %+v", m)
	}
	if res.LatestSeq != 12 {
		t.Fatalf("expected LatestSeq=12, got %d", res.LatestSeq)
	}
}

func TestTursoBackendReportsHTTPError(t *testing.T) {
	backend := newTursoTestBackend(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"message":"bad token"}`)),
			Header:     make(http.Header),
		}, nil
	}))
	_, err := backend.PullMutations(0, 1)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected '401' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("expected error body in error, got: %v", err)
	}
}

func TestTursoBackendPushUnknownEntityReturnsError(t *testing.T) {
	httpCalled := false
	backend := newTursoTestBackend(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		httpCalled = true
		return nil, nil
	}))
	_, err := backend.PushMutations([]MutationEntry{
		{LocalSeq: 1, Entity: "unknown-entity", EntityKey: "k", Op: "upsert",
			Payload: json.RawMessage(`{}`)},
	})
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
	if httpCalled {
		t.Fatal("expected no HTTP call for unknown entity")
	}
}
