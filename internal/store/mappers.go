package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func nullablePtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}

func sqlNullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func sqlNullStringPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func sqlNullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value != 0}
}

func dbString(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprint(value)
	}
}

func provenanceFromDB(row dbgen.GetProvenanceContextRow) Provenance {
	return Provenance{
		ID:                 row.ID,
		AgentID:            row.AgentID,
		AgentDisplayName:   row.AgentDisplayName,
		AgentKind:          row.AgentKind,
		SourceKindID:       row.SourceKindID,
		SourceDisplayName:  row.SourceDisplayName,
		ToolID:             row.ToolID,
		ToolDisplayName:    row.ToolDisplayName,
		ModelID:            row.ModelID,
		ModelProvider:      row.ModelProvider,
		ModelDisplayName:   row.ModelDisplayName,
		MCPClientID:        row.McpClientID,
		MCPClientName:      row.McpClientName,
		MCPClientVersion:   row.McpClientVersion,
		MCPClientTransport: row.McpClientTransport,
		CreatedAt:          row.CreatedAt,
	}
}

func provenanceInputFromStored(provenance *Provenance, fallback ProvenanceInput) ProvenanceInput {
	if provenance == nil {
		return fallback
	}
	return ProvenanceInput{
		AgentID:          provenance.AgentID,
		SourceKindID:     provenance.SourceKindID,
		ToolID:           provenance.ToolID,
		ModelID:          provenance.ModelID,
		ModelProvider:    provenance.ModelProvider,
		MCPClientID:      provenance.MCPClientID,
		MCPClientName:    provenance.MCPClientName,
		MCPClientVersion: provenance.MCPClientVersion,
		MCPTransport:     provenance.MCPClientTransport,
	}
}

func jsonStrings(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func observationFromDB(
	id int64,
	syncID any,
	sessionID, typ, title, content string,
	toolName, project sql.NullString,
	scope string,
	topicKey sql.NullString,
	revisionCount, duplicateCount int64,
	lastSeenAt sql.NullString,
	createdAt, updatedAt string,
	deletedAt sql.NullString,
) Observation {
	return Observation{
		ID:             id,
		SyncID:         dbString(syncID),
		SessionID:      sessionID,
		Type:           typ,
		Title:          title,
		Content:        content,
		ToolName:       nullablePtr(toolName),
		Project:        nullablePtr(project),
		Scope:          scope,
		TopicKey:       nullablePtr(topicKey),
		RevisionCount:  int(revisionCount),
		DuplicateCount: int(duplicateCount),
		LastSeenAt:     nullablePtr(lastSeenAt),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		DeletedAt:      nullablePtr(deletedAt),
	}
}

func observationFromListRow(r dbgen.ListObservationsRow) Observation {
	return observationFromDB(r.ID, r.SyncID, r.SessionID, r.Type, r.Title, r.Content,
		r.ToolName, r.Project, r.Scope, r.TopicKey, r.RevisionCount, r.DuplicateCount,
		r.LastSeenAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func observationFromRecentRow(r dbgen.ListRecentObservationsRow) Observation {
	return observationFromDB(r.ID, r.SyncID, r.SessionID, r.Type, r.Title, r.Content,
		r.ToolName, r.Project, r.Scope, r.TopicKey, r.RevisionCount, r.DuplicateCount,
		r.LastSeenAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func observationFromSessionRow(r dbgen.ListSessionObservationsRow) Observation {
	return observationFromDB(r.ID, r.SyncID, r.SessionID, r.Type, r.Title, r.Content,
		r.ToolName, r.Project, r.Scope, r.TopicKey, r.RevisionCount, r.DuplicateCount,
		r.LastSeenAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func searchResultFromFilterRow(r dbgen.SearchObservationsByFilterRow) SearchResult {
	return SearchResult{
		Observation: observationFromDB(r.ID, r.SyncID, r.SessionID, r.Type, r.Title, r.Content,
			r.ToolName, r.Project, r.Scope, r.TopicKey, r.RevisionCount, r.DuplicateCount,
			r.LastSeenAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt),
		Rank: r.Relevance,
	}
}

func searchResultFromFTSRow(r dbgen.SearchObservationsFTSRow) SearchResult {
	return SearchResult{
		Observation: observationFromDB(r.ID, r.SyncID, r.SessionID, r.Type, r.Title, r.Content,
			r.ToolName, r.Project, r.Scope, r.TopicKey, r.RevisionCount, r.DuplicateCount,
			r.LastSeenAt, r.CreatedAt, r.UpdatedAt, r.DeletedAt),
		Rank: r.Relevance,
	}
}

func syncStateFromDB(row dbgen.SyncState) *SyncState {
	return &SyncState{
		TargetKey: row.TargetKey, Lifecycle: row.Lifecycle,
		LastEnqueuedSeq: row.LastEnqueuedSeq, LastAckedSeq: row.LastAckedSeq,
		LastPulledSeq: row.LastPulledSeq, ConsecutiveFailures: int(row.ConsecutiveFailures),
		BackoffUntil: nullablePtr(row.BackoffUntil), LeaseOwner: nullablePtr(row.LeaseOwner),
		LeaseUntil: nullablePtr(row.LeaseUntil), LastError: nullablePtr(row.LastError),
		UpdatedAt: row.UpdatedAt,
	}
}

func observationPayloadFromObservation(obs *Observation) syncObservationPayload {
	tags := obs.Tags
	if tags == nil {
		tags = []string{}
	}
	return syncObservationPayload{
		SyncID:     obs.SyncID,
		SessionID:  obs.SessionID,
		Type:       obs.Type,
		Title:      obs.Title,
		Content:    obs.Content,
		ToolName:   obs.ToolName,
		Project:    obs.Project,
		Scope:      obs.Scope,
		TopicKey:   obs.TopicKey,
		Tags:       &tags,
		Provenance: nullableProvenanceInput(provenanceInputFromStored(obs.Provenance, ProvenanceInput{})),
	}
}
