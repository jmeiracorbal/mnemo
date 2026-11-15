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
	case protocol.OperationLimitsGet:
		result = protocol.LimitsOutput{MaxObservationLength: s.MaxObservationLength()}
	case protocol.OperationStatsGet:
		stats, err := s.Stats()
		if err != nil {
			return nil, err
		}
		result = protocol.StatsOutput{
			TotalSessions: stats.TotalSessions,
			TotalObservations: stats.TotalObservations,
			TotalPrompts: stats.TotalPrompts,
			Projects: stats.Projects,
		}
	case protocol.OperationObservationSearch:
		var input protocol.ObservationSearchInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid observation.search input"}
			break
		}
		rows, err := s.Search(input.Query, SearchOptions{
			Project: input.ProjectID, Type: input.Type, Scope: input.Scope,
			TopicKey: input.TopicKey, Tags: input.Tags, PreferTags: input.PreferTags,
			Limit: input.Limit,
		})
		if err != nil { return nil, err }
		found := make([]protocol.ObservationMatch, 0, len(rows))
		for _, item := range rows {
			if item.Project == nil {
				return nil, fmt.Errorf("search result without project")
			}
			ref, err := protocol.NewObservationRef(*item.Project, item.SyncID)
			if err != nil { return nil, err }
			found = append(found, protocol.ObservationMatch{
				Observation: projectObservation(ref, &item.Observation),
				Rank: item.Rank,
			})
		}
		result = protocol.ObservationSearchOutput{Matches: found}
	case protocol.OperationObservationUpdate:
		var input protocol.ObservationUpdateInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid observation.update input"}
			break
		}
		id, code, err := s.protocolObservationID(ctx, input.Ref)
		if err != nil {
			response.Error = &protocol.Error{Code: code, Message: err.Error()}
			break
		}
		change := input.Changes
		if change.Type == nil && change.Title == nil && change.Content == nil &&
			change.Scope == nil && change.TopicKey == nil && change.Tags == nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "observation.update requires at least one change"}
			break
		}
		updated, err := s.UpdateObservation(id, UpdateObservationParams{
			Type: change.Type, Title: change.Title, Content: change.Content,
			Scope: change.Scope, TopicKey: change.TopicKey, Tags: change.Tags,
		})
		if err != nil { return nil, err }
		result = protocol.ObservationGetOutput{Observation: projectObservation(input.Ref, updated)}
	case protocol.OperationObservationDelete:
		var input protocol.ObservationDeleteInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid observation.delete input"}
			break
		}
		id, code, err := s.protocolObservationID(ctx, input.Ref)
		if err != nil {
			response.Error = &protocol.Error{Code: code, Message: err.Error()}
			break
		}
		if err := s.DeleteObservation(id); err != nil { return nil, err }
		result = struct{}{}
	case protocol.OperationObservationTimeline:
		var input protocol.ObservationTimelineInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid observation.timeline input"}
			break
		}
		id, code, err := s.protocolObservationID(ctx, input.Ref)
		if err != nil {
			response.Error = &protocol.Error{Code: code, Message: err.Error()}
			break
		}
		timeline, err := s.Timeline(id, input.Before, input.After)
		if err != nil { return nil, err }
		out := protocol.ObservationTimelineOutput{
			Focus: projectObservation(input.Ref, &timeline.Focus),
			TotalInRange: timeline.TotalInRange,
			SessionID: timeline.Focus.SessionID,
		}
		if timeline.SessionInfo != nil {
			out.SessionStart = timeline.SessionInfo.StartedAt
			if timeline.SessionInfo.Summary != nil { out.SessionSummary = *timeline.SessionInfo.Summary }
		}
		for _, item := range timeline.Before {
			ref, err := s.protocolTimelineRef(input.Ref.ProjectID, item.ID)
			if err != nil { return nil, err }
			out.Before = append(out.Before, projectTimelineObservation(ref, &item))
		}
		for _, item := range timeline.After {
			ref, err := s.protocolTimelineRef(input.Ref.ProjectID, item.ID)
			if err != nil { return nil, err }
			out.After = append(out.After, projectTimelineObservation(ref, &item))
		}
		result = out
	case protocol.OperationContextBuild:
		var input protocol.ContextBuildInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid context.build input"}
			break
		}
		value, err := s.FormatContextOpts(input.ProjectID, input.Scope, ContextOptions{
			TopicKey: input.TopicKey, Tags: input.Tags, PreferTags: input.PreferTags,
		})
		if err != nil { return nil, err }
		result = protocol.ContextBuildOutput{Text: value}
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
	sessionID := input.SessionID
	if sessionID != "" {
		// An MCP adapter has already resolved the agent's Mnemo session.
		// Never redirect it into a synthetic session or another project.
		session, err := s.GetSession(sessionID)
		if err != nil {
			return protocol.ResourceRef{}, fmt.Errorf("lookup observation session: %w", err)
		}
		if session.Project != input.ProjectID {
			return protocol.ResourceRef{}, fmt.Errorf("observation session belongs to another project")
		}
	} else {
		// Standalone callers without an agent session use one Mnemo-owned
		// persistent session per project. No consumer or harness owns it.
		sessionID = "mnemo-protocol-" + input.ProjectID
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
	}
	id, err := s.AddObservation(AddObservationParams{
		SessionID: sessionID, Project: input.ProjectID, Type: input.Type,
		Title: input.Title, Content: input.Content, Scope: input.Scope,
		TopicKey: input.TopicKey, Tags: input.Tags,
		Provenance: ProvenanceInput{
			AgentID: input.Provenance.AgentID, SourceKindID: input.Provenance.SourceKindID,
			ToolID: input.Provenance.ToolID, ModelID: input.Provenance.ModelID,
			ModelProvider: input.Provenance.ModelProvider,
			MCPClientID: input.Provenance.MCPClientID,
			MCPClientName: input.Provenance.MCPClientName,
			MCPClientVersion: input.Provenance.MCPClientVersion,
			MCPTransport: input.Provenance.MCPTransport,
		},
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
	var toolName string
	if obs.ToolName != nil {
		toolName = *obs.ToolName
	}
	return protocol.Observation{
		Ref: ref, SessionID: obs.SessionID,
		Type: obs.Type, Title: obs.Title, Content: obs.Content, ToolName: toolName,
		Scope: obs.Scope, TopicKey: topic, Tags: obs.Tags,
		RevisionCount: obs.RevisionCount, DuplicateCount: obs.DuplicateCount,
		CreatedAt: obs.CreatedAt, UpdatedAt: obs.UpdatedAt,
	}
}


func (s *Store) protocolObservationID(ctx context.Context, ref protocol.ResourceRef) (int64, protocol.ErrorCode, error) {
	if err := ref.Validate(); err != nil { return 0, protocol.CodeInvalidReference, err }
	row, err := s.q.GetLiveObservationBySyncID(ctx, sql.NullString{String: ref.SyncID, Valid: true})
	if err == sql.ErrNoRows { return 0, protocol.CodeNotFound, fmt.Errorf("observation not found") }
	if err != nil { return 0, protocol.CodeInternal, err }
	if row.Project != ref.ProjectID {
		return 0, protocol.CodeProjectMismatch, fmt.Errorf("observation does not belong to the specified project")
	}
	return row.ID, "", nil
}

func projectTimelineObservation(ref protocol.ResourceRef, entry *TimelineEntry) protocol.Observation {
	topic, tool := "", ""
	if entry.TopicKey != nil { topic = *entry.TopicKey }
	if entry.ToolName != nil { tool = *entry.ToolName }
	return protocol.Observation{
		Ref: ref, SessionID: entry.SessionID, Type: entry.Type,
		Title: entry.Title, Content: entry.Content, ToolName: tool,
		Scope: entry.Scope, TopicKey: topic,
		RevisionCount: entry.RevisionCount, DuplicateCount: entry.DuplicateCount,
		CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
}

func (s *Store) protocolTimelineRef(projectID string, id int64) (protocol.ResourceRef, error) {
	obs, err := s.GetObservation(id)
	if err != nil { return protocol.ResourceRef{}, err }
	if obs.Project == nil || *obs.Project != projectID {
		return protocol.ResourceRef{}, fmt.Errorf("timeline observation belongs to a different project")
	}
	return protocol.NewObservationRef(projectID, obs.SyncID)
}
