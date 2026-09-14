package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

const (
	EventExecutionStarted     = "execution.started"
	EventExecutionClosed      = "execution.closed"
	EventSessionCompacted     = "session.compacted"
	EventWorkspaceFileChanged = "workspace.file_changed"
	EventGitCommitCreated     = "git.commit_created"
)

// DurableEvent is the controller input persisted by the event broker. EventID
// is globally unique and is the idempotency key; no delivery is acknowledged
// until ApplyDurableEvent commits it with its effect.
type DurableEvent struct {
	ID           string
	Type         string
	Project      string
	ExecutionKey string
	Payload      json.RawMessage
}

// BindExecutionSession records the controller-owned session selected by an MCP
// runtime for one opaque adapter execution key. Rebinding is intentional: a new
// MCP runtime for the same execution supersedes the prior runtime's session.
func (s *Store) BindExecutionSession(project, executionKey, sessionID string) error {
	if strings.TrimSpace(project) == "" {
		return fmt.Errorf("project id must not be empty")
	}
	if strings.TrimSpace(executionKey) == "" {
		return fmt.Errorf("execution key must not be empty")
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id must not be empty")
	}
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := s.q.WithTx(tx).GetSessionPayload(context.Background(), sessionID); err != nil {
			return fmt.Errorf("resolve session for execution key: %w", err)
		}
		_, err := s.execHook(tx, `
INSERT INTO execution_sessions (project, execution_key, session_id)
VALUES (?, ?, ?)
ON CONFLICT(project, execution_key) DO UPDATE SET session_id = excluded.session_id`, project, executionKey, sessionID)
		return err
	})
}

// ApplyDurableEvent applies one typed broker event and its idempotency marker in
// the same SQLite transaction. It is intentionally only called by the event
// controller; hooks and publishers never open the store.
func (s *Store) ApplyDurableEvent(event DurableEvent) error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Type) == "" {
		return fmt.Errorf("event id and type must not be empty")
	}
	if strings.TrimSpace(event.Project) == "" || strings.TrimSpace(event.ExecutionKey) == "" {
		return fmt.Errorf("event project and execution key must not be empty")
	}

	return s.withTx(func(tx *sql.Tx) error {
		result, err := s.execHook(tx, `
INSERT INTO processed_events (id, event_type, project, execution_key)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`, event.ID, event.Type, event.Project, event.ExecutionKey)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check durable event insertion: %w", err)
		}
		if changed == 0 {
			return nil
		}

		q := s.q.WithTx(tx)
		if event.Type == EventExecutionStarted {
			var payload struct {
				Directory string `json:"directory"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil || strings.TrimSpace(payload.Directory) == "" {
				return fmt.Errorf("%s payload requires directory", event.Type)
			}
			sessionID := "execution-" + event.ExecutionKey
			if err := s.createSessionTx(tx, sessionID, event.Project, payload.Directory, ProvenanceInput{}); err != nil {
				return err
			}
			_, err := s.execHook(tx, `INSERT INTO execution_sessions (project, execution_key, session_id) VALUES (?, ?, ?) ON CONFLICT(project, execution_key) DO UPDATE SET session_id = excluded.session_id`, event.Project, event.ExecutionKey, sessionID)
			return err
		}
		var sessionID string
		if err := tx.QueryRow(`SELECT session_id FROM execution_sessions WHERE project = ? AND execution_key = ?`, event.Project, event.ExecutionKey).Scan(&sessionID); err != nil {
			return fmt.Errorf("resolve event session: %w", err)
		}
		switch event.Type {
		case EventExecutionClosed:
			return q.EndSession(context.Background(), dbgen.EndSessionParams{ID: sessionID})
		case EventSessionCompacted:
			if !json.Valid(event.Payload) {
				return fmt.Errorf("invalid %s payload", event.Type)
			}
			return q.TouchSessionCompact(context.Background(), sessionID)
		case EventWorkspaceFileChanged:
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
		case EventGitCommitCreated:
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
