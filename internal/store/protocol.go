package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmeiracorbal/mnemo/v2/protocol"
	"github.com/jmeiracorbal/mnemo/v2/internal/adapters/agents"
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
	case protocol.OperationSessionResolve:
		var input protocol.SessionResolveInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid session.resolve input"}
			break
		}
		if input.ProjectID == "" || input.Directory == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "project_id and directory are required"}
			break
		}
		mcpIdentity := input.InstanceID != ""
		nativeIdentity := input.AgentID != "" && input.NativeID != ""
		if mcpIdentity == nativeIdentity {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "choose exactly one of instance_id or agent_id and native_id"}
			break
		}
		var sessionID string
		var err error
		if mcpIdentity {
			sessionID, err = s.ResolveMCPInstanceSession(input.ProjectID, input.Directory, input.InstanceID, input.PID)
		} else {
			var identity agents.Identity
			identity, err = agents.NewIdentity(agents.Agent(input.AgentID), input.ProjectID, input.NativeID)
			if err == nil { sessionID, err = s.ensureExecutionSession(identity, input.Directory) }
		}
		if err != nil { return nil, err }
		result = protocol.SessionResolveOutput{SessionID: sessionID}
	case protocol.OperationSessionClose:
		var input protocol.SessionCloseInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid session.close input"}
			break
		}
		if input.InstanceID == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "instance_id is required"}
			break
		}
		if err := s.CloseMCPInstanceSessions(input.InstanceID); err != nil { return nil, err }
		result = struct{}{}
	case protocol.OperationSessionBindExecution:
		var input protocol.SessionBindExecutionInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid session.bind_execution input"}
			break
		}
		if input.ProjectID == "" || input.NativeID == "" || input.SessionID == "" || input.AgentID == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "project_id, agent_id, native_id and session_id are required"}
			break
		}
		if err := s.BindExecutionSession(input.ProjectID, agents.Agent(input.AgentID), input.NativeID, input.SessionID); err != nil {
			return nil, err
		}
		result = struct{}{}
	case protocol.OperationSessionExists, protocol.OperationSessionObservationCount, protocol.OperationSessionProjectObservationCount:
		var input protocol.SessionIDInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.SessionID == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "session_id is required"}
			break
		}
		switch request.Operation {
		case protocol.OperationSessionExists:
			_, err := s.GetSession(input.SessionID)
			result = protocol.SessionExistsOutput{Exists: err == nil}
		case protocol.OperationSessionObservationCount:
			count, err := s.ObsCount(input.SessionID)
			if err != nil { return nil, err }
			result = protocol.SessionCountOutput{Count: count}
		case protocol.OperationSessionProjectObservationCount:
			count, err := s.ObsCountForSession(input.SessionID)
			if err != nil { return nil, err }
			result = protocol.SessionCountOutput{Count: count}
		}
	case protocol.OperationTagList:
		var input protocol.TagListInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid tag.list input"}
			break
		}
		tags, err := s.ListTags(input.ProjectID)
		if err != nil { return nil, err }
		result = protocol.TagListOutput{Tags: publicTags(tags)}
	case protocol.OperationTagStats:
		var input protocol.TagStatsInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid tag.stats input"}
			break
		}
		opt := input.Options
		tags, err := s.TagStats(input.ProjectID, TagStatsOptions{
			MinCount: opt.MinCount, MaxCount: opt.MaxCount,
			UnusedSince: opt.UnusedSince, Limit: opt.Limit, SortBy: opt.SortBy,
		})
		if err != nil { return nil, err }
		result = protocol.TagStatsOutput{Tags: publicTags(tags)}
	case protocol.OperationTagRelated:
		var input protocol.TagRelatedInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid tag.related input"}
			break
		}
		opt := input.Options
		tags, err := s.RelatedTags(input.ProjectID, input.Tag, RelatedTagsOptions{
			Limit: opt.Limit, Since: opt.Since, MinCooccurrence: opt.MinCooccurrence,
			IncludeObservations: opt.IncludeObservations, IncludeSessions: opt.IncludeSessions,
		})
		if err != nil { return nil, err }
		out := make([]protocol.RelatedTag, 0, len(tags))
		for _, tag := range tags {
			out = append(out, protocol.RelatedTag{
				Tag: tag.Tag, CooccurrenceCount: tag.CooccurrenceCount,
				Score: tag.Score, LastSeenAt: tag.LastSeenAt,
			})
		}
		result = protocol.TagRelatedOutput{Tags: out}
	case protocol.OperationTagMerge:
		var input protocol.TagMergeInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid tag.merge input"}
			break
		}
		if input.From == "" || input.To == "" || input.From == input.To {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "two distinct tag names are required"}
			break
		}
		observations, sessions, err := s.MergeTags(input.From, input.To)
		if err != nil { return nil, err }
		result = protocol.TagMergeOutput{Observations: observations, Sessions: sessions, CanonicalTo: NormalizeTag(input.To)}
	case protocol.OperationPromptCreate:
		var input protocol.PromptCreateInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.ProjectID == "" || input.SessionID == "" || input.Content == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "project_id, session_id and content are required"}
			break
		}
		session, err := s.GetSession(input.SessionID)
		if err != nil || session.Project != input.ProjectID {
			response.Error = &protocol.Error{Code: protocol.CodeProjectMismatch, Message: "prompt session does not belong to specified project"}
			break
		}
		if _, err := s.AddPrompt(AddPromptParams{
			SessionID: input.SessionID, Project: input.ProjectID, Content: input.Content,
			Provenance: provenanceFromPublic(input.Provenance),
		}); err != nil { return nil, err }
		result = protocol.PromptCreateOutput{Created: true}
	case protocol.OperationCapturePassive:
		var input protocol.CapturePassiveInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.ProjectID == "" || input.SessionID == "" || input.Content == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "project_id, session_id and content are required"}
			break
		}
		session, err := s.GetSession(input.SessionID)
		if err != nil || session.Project != input.ProjectID {
			response.Error = &protocol.Error{Code: protocol.CodeProjectMismatch, Message: "passive capture session does not belong to specified project"}
			break
		}
		captured, err := s.PassiveCapture(PassiveCaptureParams{
			SessionID: input.SessionID, Project: input.ProjectID,
			Content: input.Content, Source: input.Source,
			Provenance: provenanceFromPublic(input.Provenance),
		})
		if err != nil { return nil, err }
		result = protocol.CapturePassiveOutput{
			Extracted: captured.Extracted, Saved: captured.Saved,
			Duplicates: captured.Duplicates, SuggestedTags: captured.SuggestedTags,
		}
	case protocol.OperationProjectList:
		projects, err := s.ListProjectSummaries()
		if err != nil { return nil, err }
		out := make([]protocol.ProjectSummary, 0, len(projects))
		for _, item := range projects { out = append(out, publicProjectSummary(item)) }
		result = protocol.ProjectListOutput{Projects: out}
	case protocol.OperationProjectMergePlan, protocol.OperationProjectMerge:
		var input protocol.ProjectPairInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.From == "" || input.To == "" || input.From == input.To {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "distinct source and destination projects are required"}
			break
		}
		if request.Operation == protocol.OperationProjectMergePlan {
			plan, err := s.BuildProjectMergePlan(input.From, input.To)
			if err != nil { return nil, err }
			result = publicMergePlan(plan)
		} else {
			merged, err := s.MergeProjects(input.From, input.To)
			if err != nil { return nil, err }
			result = protocol.ProjectMergeOutput{
				Plan: publicMergePlan(&merged.Plan), Merged: merged.Merged,
				ObservationsUpdated: merged.ObservationsUpdated,
				SessionsUpdated: merged.SessionsUpdated, PromptsUpdated: merged.PromptsUpdated,
				SourceProjectDeleted: merged.SourceProjectDeleted,
			}
		}
	case protocol.OperationProjectRenamePlan, protocol.OperationProjectRename:
		var input protocol.ProjectRenameInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.Name == "" || (input.Selector.ID == "" && input.Selector.Path == "") {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "project selector and new name are required"}
			break
		}
		selector := ProjectRenameSelector{ID: input.Selector.ID, Path: input.Selector.Path}
		if request.Operation == protocol.OperationProjectRenamePlan {
			plan, err := s.BuildProjectRenamePlan(selector, input.Name)
			if err != nil { return nil, err }
			result = publicRenamePlan(plan)
		} else {
			renamed, err := s.RenameProject(selector, input.Name)
			if err != nil { return nil, err }
			result = protocol.ProjectRenameOutput{Plan: publicRenamePlan(&renamed.Plan), Renamed: renamed.Renamed}
		}
	case protocol.OperationProjectIdentityMigrate:
		var input protocol.ProjectIdentityMigrateInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.LegacyKey == "" || input.ProjectID == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "legacy_key and project_id are required"}
			break
		}
		migrated, err := s.MigrateProject(input.LegacyKey, input.ProjectID)
		if err != nil { return nil, err }
		if err := s.EnsureProject(input.ProjectID, input.Name); err != nil { return nil, err }
		result = protocol.ProjectIdentityMigrateOutput{
			Migrated: migrated.Migrated, ObservationsUpdated: migrated.ObservationsUpdated,
			SessionsUpdated: migrated.SessionsUpdated, PromptsUpdated: migrated.PromptsUpdated,
		}
	case protocol.OperationMemoryReview:
		var input protocol.MemoryReviewInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid memory.review input"}
			break
		}
		report, err := s.ReviewMemoryConflicts(MemoryReviewOptions{
			Project: input.ProjectID, Scope: input.Scope,
			TopicKey: input.TopicKey, Limit: input.Limit,
		})
		if err != nil { return nil, err }
		groups := make([]protocol.MemoryConflictGroup, 0, len(report.Groups))
		for _, group := range report.Groups {
			converted := protocol.MemoryConflictGroup{
				ID: group.ID, Kind: group.Kind, Reason: group.Reason,
				Confidence: group.Confidence, ProjectID: group.Project,
				Scope: group.Scope, TopicKey: group.TopicKey, SuggestedAction: group.SuggestedAction,
			}
			for _, item := range group.Observations {
				ref, err := s.protocolRefForID(item.ID)
				if err != nil { return nil, err }
				entry := protocol.MemoryReviewObservation{
					Ref: ref, Type: item.Type, Title: item.Title,
					Scope: item.Scope, TopicKey: item.TopicKey,
					CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
				}
				if item.ReviewState != nil {
					state := &protocol.MemoryReviewState{
						State: item.ReviewState.State, Reason: item.ReviewState.Reason,
						ReviewedAt: item.ReviewState.ReviewedAt, UpdatedAt: item.ReviewState.UpdatedAt,
					}
					if item.ReviewState.SupersededBy != nil {
						superseded, err := s.protocolRefForID(*item.ReviewState.SupersededBy)
						if err != nil { return nil, err }
						state.SupersededBy = &superseded
					}
					entry.ReviewState = state
				}
				converted.Observations = append(converted.Observations, entry)
			}
			groups = append(groups, converted)
		}
		result = protocol.MemoryReviewOutput{Total: report.Total, Groups: groups}
	case protocol.OperationMemoryMarkReviewed, protocol.OperationMemoryMarkStale:
		var input protocol.MemoryMarkInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid memory marking input"}
			break
		}
		id, code, err := s.protocolObservationID(ctx, input.Ref)
		if err != nil { response.Error = &protocol.Error{Code: code, Message: err.Error()}; break }
		if request.Operation == protocol.OperationMemoryMarkReviewed {
			err = s.MarkMemoryReviewed(id, input.Reason)
		} else {
			err = s.MarkMemoryStale(id, input.Reason)
		}
		if err != nil { return nil, err }
		result = struct{}{}
	case protocol.OperationMemorySupersede:
		var input protocol.MemorySupersedeInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid memory.supersede input"}
			break
		}
		if input.OldRef.ProjectID != input.NewRef.ProjectID {
			response.Error = &protocol.Error{Code: protocol.CodeProjectMismatch, Message: "superseded memories must belong to the same Mnemo project"}
			break
		}
		oldID, code, err := s.protocolObservationID(ctx, input.OldRef)
		if err != nil { response.Error = &protocol.Error{Code: code, Message: err.Error()}; break }
		newID, code, err := s.protocolObservationID(ctx, input.NewRef)
		if err != nil { response.Error = &protocol.Error{Code: code, Message: err.Error()}; break }
		if err := s.SupersedeMemory(oldID, newID, input.Reason); err != nil { return nil, err }
		result = struct{}{}
	case protocol.OperationMemoryConsolidationPlan, protocol.OperationMemoryConsolidate:
		var input protocol.MemoryConsolidationInput
		if err := json.Unmarshal(request.Payload, &input); err != nil || input.FromTopic == "" || input.ToTopic == "" {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "from_topic and to_topic are required"}
			break
		}
		var plan *MemoryTopicConsolidationPlan
		var err error
		if request.Operation == protocol.OperationMemoryConsolidationPlan {
			plan, err = s.PlanMemoryTopicConsolidation(input.FromTopic, input.ToTopic, input.ProjectID, input.Scope)
		} else {
			plan, err = s.ConsolidateMemoryTopic(input.FromTopic, input.ToTopic, input.ProjectID, input.Scope)
		}
		if err != nil { return nil, err }
		out := protocol.MemoryConsolidationOutput{
			FromTopic: plan.FromTopic, ToTopic: plan.ToTopic,
			ProjectID: plan.Project, Scope: plan.Scope, Total: plan.Total,
		}
		for _, id := range plan.IDs {
			ref, err := s.protocolRefForID(id)
			if err != nil { return nil, err }
			out.Refs = append(out.Refs, ref)
		}
		result = out
	case protocol.OperationArchiveExport:
		data, err := s.Export()
		if err != nil { return nil, err }
		out := protocol.ArchiveExportOutput{Version: protocol.Version, ExportedAt: data.ExportedAt}
		for _, sess := range data.Sessions {
			out.Sessions = append(out.Sessions, protocol.ArchiveSession{
				SessionID: sess.ID, ProjectID: sess.Project, Directory: sess.Directory,
				StartedAt: sess.StartedAt, EndedAt: sess.EndedAt, Summary: sess.Summary,
				IsDeleted: sess.IsDeleted, Tags: sess.Tags, Provenance: archiveProvenance(sess.Provenance),
			})
		}
		for _, obs := range data.Observations {
			if obs.Project == nil {
				return nil, fmt.Errorf("export observation has no Mnemo project")
			}
			ref, err := protocol.NewObservationRef(*obs.Project, obs.SyncID)
			if err != nil { return nil, err }
			out.Observations = append(out.Observations, protocol.ArchiveObservation{
				Ref: ref, SessionID: obs.SessionID, Type: obs.Type,
				Title: obs.Title, Content: obs.Content, ToolName: obs.ToolName,
				Scope: obs.Scope, TopicKey: obs.TopicKey, Tags: obs.Tags,
				RevisionCount: obs.RevisionCount, DuplicateCount: obs.DuplicateCount,
				LastSeenAt: obs.LastSeenAt, CreatedAt: obs.CreatedAt,
				UpdatedAt: obs.UpdatedAt, IsDeleted: obs.IsDeleted,
				Provenance: archiveProvenance(obs.Provenance),
			})
		}
		for _, prompt := range data.Prompts {
			out.Prompts = append(out.Prompts, protocol.ArchivePrompt{
				ProjectID: prompt.Project, SyncID: prompt.SyncID, SessionID: prompt.SessionID,
				Content: prompt.Content, CreatedAt: prompt.CreatedAt, IsDeleted: prompt.IsDeleted,
				Provenance: archiveProvenance(prompt.Provenance),
			})
		}
		result = out
	case protocol.OperationArchiveImport:
		var input protocol.ArchiveImportInput
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			response.Error = &protocol.Error{Code: protocol.CodeInvalidRequest, Message: "invalid archive.import input"}
			break
		}
		if input.Archive.Version != protocol.Version {
			response.Error = &protocol.Error{Code: protocol.CodeUnsupportedVersion, Message: "unsupported Mnemo archive version"}
			break
		}
		data := &ExportData{Version: input.Archive.Version, ExportedAt: input.Archive.ExportedAt}
		for _, sess := range input.Archive.Sessions {
			data.Sessions = append(data.Sessions, Session{
				ID: sess.SessionID, Project: sess.ProjectID, Directory: sess.Directory,
				StartedAt: sess.StartedAt, EndedAt: sess.EndedAt, Summary: sess.Summary,
				IsDeleted: sess.IsDeleted, Tags: sess.Tags,
				Provenance: archiveStoredProvenance(sess.Provenance),
			})
		}
		for _, obs := range input.Archive.Observations {
			if err := obs.Ref.Validate(); err != nil {
				response.Error = &protocol.Error{Code: protocol.CodeInvalidReference, Message: err.Error()}
				break
			}
			project := obs.Ref.ProjectID
			data.Observations = append(data.Observations, Observation{
				SyncID: obs.Ref.SyncID, SessionID: obs.SessionID, Project: &project,
				Type: obs.Type, Title: obs.Title, Content: obs.Content,
				ToolName: obs.ToolName, Scope: obs.Scope, TopicKey: obs.TopicKey,
				Tags: obs.Tags, RevisionCount: obs.RevisionCount,
				DuplicateCount: obs.DuplicateCount, LastSeenAt: obs.LastSeenAt,
				CreatedAt: obs.CreatedAt, UpdatedAt: obs.UpdatedAt,
				IsDeleted: obs.IsDeleted,
				Provenance: archiveStoredProvenance(obs.Provenance),
			})
		}
		if response.Error != nil { break }
		for _, prompt := range input.Archive.Prompts {
			data.Prompts = append(data.Prompts, Prompt{
				Project: prompt.ProjectID, SyncID: prompt.SyncID, SessionID: prompt.SessionID,
				Content: prompt.Content, CreatedAt: prompt.CreatedAt,
				IsDeleted: prompt.IsDeleted,
				Provenance: archiveStoredProvenance(prompt.Provenance),
			})
		}
		imported, err := s.Import(data)
		if err != nil { return nil, err }
		result = protocol.ArchiveImportOutput{
			SessionsImported: imported.SessionsImported,
			ObservationsImported: imported.ObservationsImported,
			PromptsImported: imported.PromptsImported,
		}
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

func publicTags(tags []TagInfo) []protocol.TagInfo {
	out := make([]protocol.TagInfo, 0, len(tags))
	for _, tag := range tags {
		out = append(out, protocol.TagInfo{
			Tag: tag.Tag, Count: tag.Count, LastUsedAt: tag.LastUsedAt,
		})
	}
	return out
}

func provenanceFromPublic(input protocol.Provenance) ProvenanceInput {
	return ProvenanceInput{
		AgentID: input.AgentID, SourceKindID: input.SourceKindID,
		ToolID: input.ToolID, ModelID: input.ModelID, ModelProvider: input.ModelProvider,
		MCPClientID: input.MCPClientID, MCPClientName: input.MCPClientName,
		MCPClientVersion: input.MCPClientVersion, MCPTransport: input.MCPTransport,
	}
}

func publicProjectSummary(s ProjectSummary) protocol.ProjectSummary {
	return protocol.ProjectSummary{
		ID: s.ID, Name: s.Name, CreatedAt: s.CreatedAt, Directory: s.Directory,
		SessionCount: s.SessionCount, ObservationCount: s.ObservationCount,
		PromptCount: s.PromptCount, LastSeenAt: s.LastSeenAt,
	}
}

func publicMergePlan(plan *ProjectMergePlan) protocol.ProjectMergePlan {
	return protocol.ProjectMergePlan{
		From: publicProjectSummary(plan.From), To: publicProjectSummary(plan.To),
		Observations: plan.Observations, Sessions: plan.Sessions, Prompts: plan.Prompts,
		SourceProjectRows: plan.SourceProjectRows,
		WillDeleteSourceProject: plan.WillDeleteSourceProject,
	}
}

func publicRenamePlan(plan *ProjectRenamePlan) protocol.ProjectRenamePlan {
	return protocol.ProjectRenamePlan{
		Project: publicProjectSummary(plan.Project), Selector: plan.Selector,
		SelectorValue: plan.SelectorValue, NewName: plan.NewName,
		WillChange: plan.WillChange,
	}
}

func (s *Store) protocolRefForID(id int64) (protocol.ResourceRef, error) {
	obs, err := s.GetObservation(id)
	if err != nil { return protocol.ResourceRef{}, err }
	if obs.Project == nil { return protocol.ResourceRef{}, fmt.Errorf("observation has no project identity") }
	return protocol.NewObservationRef(*obs.Project, obs.SyncID)
}

func archiveProvenance(input *Provenance) *protocol.Provenance {
	if input == nil { return nil }
	return &protocol.Provenance{
		AgentID: input.AgentID, SourceKindID: input.SourceKindID,
		ToolID: input.ToolID, ModelID: input.ModelID, ModelProvider: input.ModelProvider,
		MCPClientID: input.MCPClientID, MCPClientName: input.MCPClientName,
		MCPClientVersion: input.MCPClientVersion,
		MCPTransport: input.MCPClientTransport,
	}
}

func archiveStoredProvenance(input *protocol.Provenance) *Provenance {
	if input == nil { return nil }
	return &Provenance{
		AgentID: input.AgentID, SourceKindID: input.SourceKindID,
		ToolID: input.ToolID, ModelID: input.ModelID, ModelProvider: input.ModelProvider,
		MCPClientID: input.MCPClientID, MCPClientName: input.MCPClientName,
		MCPClientVersion: input.MCPClientVersion,
		MCPClientTransport: input.MCPTransport,
	}
}
