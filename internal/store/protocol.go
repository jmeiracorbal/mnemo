package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmeiracorbal/mnemo/protocol"
)

// protocolFailure serializes a typed public error, not an internal RPC error.
func protocolFailure(code protocol.ErrorCode, message string) json.RawMessage {
	data, _ := json.Marshal(protocol.Response{
		Version: protocol.Version,
		Error:   &protocol.Error{Code: code, Message: message},
	})
	return data
}

// ExecuteProtocolRequest dispatches the public domain contract. It does not
// introduce another store, controller, or listener.
func (s *Store) ExecuteProtocolRequest(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var request protocol.Request
	if err := json.Unmarshal(payload, &request); err != nil {
		return protocolFailure(protocol.CodeInvalidRequest, "invalid protocol envelope"), nil
	}
	if err := request.Validate(); err != nil {
		var coded *protocol.Error
		if errors.As(err, &coded) {
			return protocolFailure(coded.Code, coded.Message), nil
		}
		return protocolFailure(protocol.CodeInvalidRequest, err.Error()), nil
	}
	response := protocol.Response{Version: protocol.Version, RequestID: request.RequestID}
	var result any
	switch request.Operation {
	case protocol.OperationHealth:
		if err := s.db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("protocol health check: %w", err)
		}
		if _, err := s.q.CountSessions(ctx); err != nil {
			return nil, fmt.Errorf("protocol health SQLite read: %w", err)
		}
		result = protocol.HealthOutput{Ready: true}
	case protocol.OperationObservationGet:
		var input protocol.ObservationGetInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid observation.get input"}
			break
		}
		if err := input.Ref.Validate(); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidReference, Message: err.Error()}
			break
		}
		observation, code, err := s.protocolGetObservation(ctx, input.Ref)
		if err != nil {
			response.Error = &protocol.Error{Code: code, Message: err.Error()}
			break
		}
		result = protocol.ObservationGetOutput{Observation: observation}
	case protocol.OperationObservationCreate:
		var input protocol.ObservationCreateInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid observation.create input"}
			break
		}
		if err := input.Validate(); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: err.Error()}
			break
		}
		ref, err := s.protocolCreateObservation(input)
		if err != nil {
			return nil, fmt.Errorf("create public observation: %w", err)
		}
		result = protocol.ObservationCreateOutput{Ref: ref}
	default:
		response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "unknown protocol operation"}
	}
	if response.Error == nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("encode protocol operation result: %w", err)
		}
		response.Payload = encoded
	}
	return json.Marshal(response)
}

func (s *Store) protocolGetObservation(ctx context.Context, ref protocol.ResourceRef) (protocol.Observation, protocol.ErrorCode, error) {
	row, err := s.q.GetLiveObservationBySyncID(ctx, sql.NullString{String: ref.SyncID, Valid: true})
	if err == sql.ErrNoRows {
		return protocol.Observation{}, protocol.CodeNotFound, fmt.Errorf("observation not found")
	}
	if err != nil {
		return protocol.Observation{}, protocol.CodeInternal, err
	}
	if row.Project != ref.ProjectID {
		return protocol.Observation{}, protocol.CodeProjectMismatch, fmt.Errorf("observation does not belong to the specified project")
	}
	obs := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
		row.ToolName, sqlNullString(row.Project), row.Scope, row.TopicKey,
		row.RevisionCount, row.DuplicateCount, row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.IsDeleted)
	if err := s.loadTagsForObservation(&obs); err != nil {
		return protocol.Observation{}, protocol.CodeInternal, err
	}
	return projectObservation(ref, &obs), "", nil
}

func (s *Store) protocolCreateObservation(input protocol.ObservationCreateInput) (protocol.ResourceRef, error) {
	// One persistent session per project for direct API writes. A Mnemo memory
	// always belongs to a real project, never to the harness that requested it.
	sessionID := "mnemo-protocol-" + input.ProjectID
	existing, err := s.q.GetSessionPayload(context.Background(), sessionID)
	if err == sql.ErrNoRows {
		if err := s.withTx(func(tx *sql.Tx) error {
			return s.createSessionTx(tx, sessionID, input.ProjectID, "mnemo/1", ProvenanceInput{})
		}); err != nil {
			return protocol.ResourceRef{}, err
		}
	} else if err != nil {
		return protocol.ResourceRef{}, err
	} else if existing.Project != input.ProjectID || existing.IsDeleted != 0 {
		return protocol.ResourceRef{}, fmt.Errorf("protocol session identity conflicts with existing session")
	}
	id, err := s.AddObservation(AddObservationParams{
		SessionID: sessionID, Project: input.ProjectID, Type: input.Type,
		Title: input.Title, Content: input.Content, Scope: input.Scope,
		TopicKey: input.TopicKey, Tags: input.Tags,
	})
	if err != nil {
		return protocol.ResourceRef{}, err
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		return protocol.ResourceRef{}, err
	}
	return protocol.NewObservationRef(input.ProjectID, obs.SyncID)
}

func projectObservation(ref protocol.ResourceRef, obs *Observation) protocol.Observation {
	var topic string
	if obs.TopicKey != nil {
		topic = *obs.TopicKey
	}
	return protocol.Observation{
		Ref: ref, Type: obs.Type, Title: obs.Title, Content: obs.Content,
		Scope: obs.Scope, TopicKey: topic, Tags: obs.Tags,
		CreatedAt: obs.CreatedAt, UpdatedAt: obs.UpdatedAt,
	}
}

