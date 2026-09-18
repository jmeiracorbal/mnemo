package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmeiracorbal/mnemo/adapters"
	"github.com/jmeiracorbal/mnemo/internal/events"
	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

// BindExecutionSession records the controller-owned session selected by an MCP
// runtime for one native adapter execution identity. The controller derives the
// opaque execution key itself so MCP clients never submit a hash. Rebinding is
// intentional: a new MCP runtime for the same execution supersedes the prior
// runtime's session.
func (s *Store) BindExecutionSession(project string, agent adapters.Agent, nativeID, sessionID string) error {
	identity, err := adapters.NewIdentity(agent, project, nativeID)
	if err != nil {
		return fmt.Errorf("derive execution identity: %w", err)
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id must not be empty")
	}
	return s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.GetSessionPayload(context.Background(), sessionID); err != nil {
			return fmt.Errorf("resolve session for execution key: %w", err)
		}
		return q.BindExecutionSessionKey(context.Background(), dbgen.BindExecutionSessionKeyParams{
			Project: project, ExecutionKey: identity.Execution, SessionID: sessionID,
		})
	})
}

// ApplyDurableEvent applies one typed broker event and its idempotency marker in
// the same SQLite transaction. It is intentionally only called by the event
// controller; hooks and publishers never open the store.
func (s *Store) ApplyDurableEvent(event events.DurableEvent) error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Type) == "" {
		return fmt.Errorf("event id and type must not be empty")
	}
	if strings.TrimSpace(event.Project) == "" || strings.TrimSpace(event.ExecutionKey) == "" {
		return fmt.Errorf("event project and execution key must not be empty")
	}

	return s.withTx(func(tx *sql.Tx) error {
		if err := s.ensureProjectTx(tx, event.Project); err != nil {
			return err
		}
		q := s.q.WithTx(tx)
		changed, err := q.InsertProcessedEvent(context.Background(), dbgen.InsertProcessedEventParams{
			ID: event.ID, EventType: event.Type, Project: event.Project, ExecutionKey: event.ExecutionKey,
		})
		if err != nil {
			return err
		}
		if changed == 0 {
			return nil
		}

		if event.Type == events.EventExecutionStarted {
			var payload struct {
				Directory string `json:"directory"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil || strings.TrimSpace(payload.Directory) == "" {
				return fmt.Errorf("%s payload requires directory", event.Type)
			}
			_, err := s.ensureExecutionSessionTx(tx, adapters.Identity{Project: event.Project, Execution: event.ExecutionKey}, payload.Directory)
			return err
		}
		sessionID, err := q.GetExecutionSessionID(context.Background(), dbgen.GetExecutionSessionIDParams{
			Project: event.Project, ExecutionKey: event.ExecutionKey,
		})
		if err != nil {
			return fmt.Errorf("resolve event session: %w", err)
		}
		switch event.Type {
		case events.EventExecutionClosed:
			return q.EndSession(context.Background(), dbgen.EndSessionParams{ID: sessionID})
		case events.EventSessionCompacted:
			if !json.Valid(event.Payload) {
				return fmt.Errorf("invalid %s payload", event.Type)
			}
			return q.TouchSessionCompact(context.Background(), sessionID)
		case events.EventAgentToolResult:
			var payload struct {
				Tool string `json:"tool"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return fmt.Errorf("decode %s payload: %w", event.Type, err)
			}
			if strings.TrimSpace(payload.Tool) == "" {
				return fmt.Errorf("%s payload requires tool", event.Type)
			}
			_, err := q.InsertObservation(context.Background(), dbgen.InsertObservationParams{
				SyncID: sqlNullString(newSyncID("event")), SessionID: sessionID,
				Type: "tool_use", Title: "Tool used: " + payload.Tool,
				Content: string(event.Payload), Scope: "project",
				NormalizedHash: sqlNullString(hashNormalized(string(event.Payload))),
			})
			return err
		case events.EventWorkspaceFileChanged:
			var payload struct {
				Path   string `json:"path"`
				Action string `json:"action"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return fmt.Errorf("decode %s payload: %w", event.Type, err)
			}
			if strings.TrimSpace(payload.Path) == "" || strings.TrimSpace(payload.Action) == "" {
				return fmt.Errorf("%s payload requires path and action", event.Type)
			}
			_, err := q.InsertObservation(context.Background(), dbgen.InsertObservationParams{
				SyncID: sqlNullString(newSyncID("event")), SessionID: sessionID,
				Type: "file_change", Title: "File " + payload.Action + ": " + payload.Path,
				Content: string(event.Payload), Scope: "project",
				NormalizedHash: sqlNullString(hashNormalized(string(event.Payload))),
			})
			return err
		case events.EventGitCommitCreated:
			var payload struct {
				Hash    string `json:"hash"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return fmt.Errorf("decode %s payload: %w", event.Type, err)
			}
			if strings.TrimSpace(payload.Hash) == "" || strings.TrimSpace(payload.Message) == "" {
				return fmt.Errorf("%s payload requires hash and message", event.Type)
			}
			_, err := q.InsertObservation(context.Background(), dbgen.InsertObservationParams{
				SyncID: sqlNullString(newSyncID("event")), SessionID: sessionID,
				Type: "decision", Title: "Git commit: " + payload.Hash,
				Content: payload.Message, Scope: "project",
				NormalizedHash: sqlNullString(hashNormalized(payload.Message)),
			})
			return err
		default:
			return fmt.Errorf("unsupported durable event type %q", event.Type)
		}
	})
}
