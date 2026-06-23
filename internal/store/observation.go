package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func (s *Store) AllObservations(project, scope string, limit int, tags ...string) ([]Observation, error) {
	if limit <= 0 {
		limit = s.cfg.MaxContextResults
	}

	normalizedTags := normalizeTagList(tags)
	rows, err := s.q.ListObservations(context.Background(), dbgen.ListObservationsParams{
		Project: project, Scope: normalizeOptionalScope(scope), TagCount: len(normalizedTags),
		TagsJson: jsonStrings(normalizedTags), ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]Observation, 0, len(rows))
	for _, row := range rows {
		results = append(results, observationFromListRow(row))
	}
	if err := s.loadTagsForObservations(results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) AddObservation(p AddObservationParams) (int64, error) {
	title := stripPrivateTags(p.Title)
	content := stripPrivateTags(p.Content)

	if len(content) > s.cfg.MaxObservationLength {
		content = content[:s.cfg.MaxObservationLength] + "... [truncated]"
	}
	scope := normalizeScope(p.Scope)
	normHash := hashNormalized(content)
	topicKey := normalizeTopicKey(p.TopicKey)

	var observationID int64
	err := s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		var obs *Observation
		if topicKey != "" {
			existingID, err := q.FindObservationByTopic(context.Background(), dbgen.FindObservationByTopicParams{
				TopicKey: sqlNullString(topicKey), Project: sqlNullString(p.Project), Scope: scope,
			})
			if err == nil {
				if err := q.UpdateObservationByTopic(context.Background(), dbgen.UpdateObservationByTopicParams{
					Type: p.Type, Title: title, Content: content, ToolName: sqlNullString(p.ToolName),
					TopicKey: sqlNullString(topicKey), NormalizedHash: sqlNullString(normHash), ID: existingID,
				}); err != nil {
					return err
				}
				observationID = existingID
				if len(p.Tags) > 0 {
					if err := s.setTagsForObservationTx(tx, existingID, p.Tags); err != nil {
						return err
					}
				}
				obs, err = s.getObservationTx(tx, existingID)
				if err != nil {
					return err
				}
				return s.enqueueSyncMutationTx(tx, SyncEntityObservation, obs.SyncID, SyncOpUpsert, observationPayloadFromObservation(obs))
			}
			if err != sql.ErrNoRows {
				return err
			}
		}

		window := dedupeWindowExpression(s.cfg.DedupeWindow)
		existingID, err := q.FindDuplicateObservation(context.Background(), dbgen.FindDuplicateObservationParams{
			NormalizedHash: sqlNullString(normHash), Project: sqlNullString(p.Project), Scope: scope,
			Type: p.Type, Title: title, Window: window,
		})
		if err == nil {
			if err := q.TouchDuplicateObservation(context.Background(), existingID); err != nil {
				return err
			}
			obs, err = s.getObservationTx(tx, existingID)
			if err != nil {
				return err
			}
			observationID = existingID
			return s.enqueueSyncMutationTx(tx, SyncEntityObservation, obs.SyncID, SyncOpUpsert, observationPayloadFromObservation(obs))
		}
		if err != sql.ErrNoRows {
			return err
		}

		syncID := newSyncID("obs")
		observationID, err = q.InsertObservation(context.Background(), dbgen.InsertObservationParams{
			SyncID: sqlNullString(syncID), SessionID: p.SessionID, Type: p.Type, Title: title, Content: content,
			ToolName: sqlNullString(p.ToolName), Project: sqlNullString(p.Project), Scope: scope,
			TopicKey: sqlNullString(topicKey), NormalizedHash: sqlNullString(normHash),
		})
		if err != nil {
			return err
		}
		if len(p.Tags) > 0 {
			if err := s.setTagsForObservationTx(tx, observationID, p.Tags); err != nil {
				return err
			}
		}
		obs, err = s.getObservationTx(tx, observationID)
		if err != nil {
			return err
		}
		return s.enqueueSyncMutationTx(tx, SyncEntityObservation, obs.SyncID, SyncOpUpsert, observationPayloadFromObservation(obs))
	})
	if err != nil {
		return 0, err
	}
	return observationID, nil
}

func (s *Store) RecentObservations(project, scope string, limit int, tags ...string) ([]Observation, error) {
	return s.recentObservations(project, scope, limit, ContextOptions{Tags: tags})
}

func (s *Store) RecentObservationsOpts(project, scope string, limit int, opts ContextOptions) ([]Observation, error) {
	return s.recentObservations(project, scope, limit, opts)
}

func (s *Store) recentObservations(project, scope string, limit int, opts ContextOptions) ([]Observation, error) {
	if limit <= 0 {
		limit = s.cfg.MaxContextResults
	}
	tags := normalizeTagList(opts.Tags)
	preferTags := normalizeTagList(opts.PreferTags)
	rows, err := s.q.ListRecentObservations(context.Background(), dbgen.ListRecentObservationsParams{
		TopicKey: strings.TrimSpace(opts.TopicKey), PreferTagsJson: jsonStrings(preferTags),
		Project: project, Scope: normalizeOptionalScope(scope), TagCount: len(tags),
		TagsJson: jsonStrings(tags), ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]Observation, 0, len(rows))
	for _, row := range rows {
		results = append(results, observationFromRecentRow(row))
	}
	if err := s.loadTagsForObservations(results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) GetObservation(id int64) (*Observation, error) {
	row, err := s.q.GetLiveObservation(context.Background(), id)
	if err != nil {
		return nil, err
	}
	o := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
		row.ToolName, row.Project, row.Scope, row.TopicKey, row.RevisionCount, row.DuplicateCount,
		row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
	if err := s.loadTagsForObservation(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) UpdateObservation(id int64, p UpdateObservationParams) (*Observation, error) {
	var updated *Observation
	err := s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		obs, err := s.getObservationTx(tx, id)
		if err != nil {
			return err
		}

		typ := obs.Type
		title := obs.Title
		content := obs.Content
		project := derefString(obs.Project)
		scope := obs.Scope
		topicKey := derefString(obs.TopicKey)

		if p.Type != nil {
			typ = *p.Type
		}
		if p.Title != nil {
			title = stripPrivateTags(*p.Title)
		}
		if p.Content != nil {
			content = stripPrivateTags(*p.Content)
			if len(content) > s.cfg.MaxObservationLength {
				content = content[:s.cfg.MaxObservationLength] + "... [truncated]"
			}
		}
		if p.Project != nil {
			project = *p.Project
		}
		if p.Scope != nil {
			scope = normalizeScope(*p.Scope)
		}
		if p.TopicKey != nil {
			topicKey = normalizeTopicKey(*p.TopicKey)
		}

		if err := q.UpdateObservationFields(context.Background(), dbgen.UpdateObservationFieldsParams{
			Type: typ, Title: title, Content: content, Project: sqlNullString(project),
			Scope: scope, TopicKey: sqlNullString(topicKey), NormalizedHash: sqlNullString(hashNormalized(content)), ID: id,
		}); err != nil {
			return err
		}

		if p.Tags != nil {
			if err := s.setTagsForObservationTx(tx, id, *p.Tags); err != nil {
				return err
			}
		}
		updated, err = s.getObservationTx(tx, id)
		if err != nil {
			return err
		}
		return s.enqueueSyncMutationTx(tx, SyncEntityObservation, updated.SyncID, SyncOpUpsert, observationPayloadFromObservation(updated))
	})
	if err != nil {
		return nil, err
	}
	if err := s.loadTagsForObservation(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) DeleteObservation(id int64, hardDelete bool) error {
	return s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		obs, err := s.getObservationTx(tx, id)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}

		deletedAt := Now()
		if hardDelete {
			if err := q.HardDeleteObservation(context.Background(), id); err != nil {
				return err
			}
		} else {
			if err := q.SoftDeleteObservation(context.Background(), id); err != nil {
				return err
			}
			value, err := q.GetObservationDeletedAt(context.Background(), id)
			if err != nil {
				return err
			}
			deletedAt = value.String
		}

		return s.enqueueSyncMutationTx(tx, SyncEntityObservation, obs.SyncID, SyncOpDelete, syncObservationPayload{
			SyncID:     obs.SyncID,
			Deleted:    true,
			DeletedAt:  &deletedAt,
			HardDelete: hardDelete,
		})
	})
}

func (s *Store) Timeline(observationID int64, before, after int) (*TimelineResult, error) {
	if before <= 0 {
		before = 5
	}
	if after <= 0 {
		after = 5
	}

	focus, err := s.GetObservation(observationID)
	if err != nil {
		return nil, fmt.Errorf("timeline: observation #%d not found: %w", observationID, err)
	}

	session, err := s.GetSession(focus.SessionID)
	if err != nil {
		session = nil
	}

	beforeRows, err := s.q.ListTimelineBefore(context.Background(), dbgen.ListTimelineBeforeParams{
		SessionID: focus.SessionID, ObservationID: observationID, ResultLimit: int64(before),
	})
	if err != nil {
		return nil, fmt.Errorf("timeline: before query: %w", err)
	}
	beforeEntries := make([]TimelineEntry, 0, len(beforeRows))
	for _, row := range beforeRows {
		beforeEntries = append(beforeEntries, TimelineEntry{
			ID: row.ID, SessionID: row.SessionID, Type: row.Type, Title: row.Title, Content: row.Content,
			ToolName: nullablePtr(row.ToolName), Project: nullablePtr(row.Project), Scope: row.Scope,
			TopicKey: nullablePtr(row.TopicKey), RevisionCount: int(row.RevisionCount),
			DuplicateCount: int(row.DuplicateCount), LastSeenAt: nullablePtr(row.LastSeenAt),
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: nullablePtr(row.DeletedAt),
		})
	}
	for i, j := 0, len(beforeEntries)-1; i < j; i, j = i+1, j-1 {
		beforeEntries[i], beforeEntries[j] = beforeEntries[j], beforeEntries[i]
	}

	afterRows, err := s.q.ListTimelineAfter(context.Background(), dbgen.ListTimelineAfterParams{
		SessionID: focus.SessionID, ObservationID: observationID, ResultLimit: int64(after),
	})
	if err != nil {
		return nil, fmt.Errorf("timeline: after query: %w", err)
	}
	afterEntries := make([]TimelineEntry, 0, len(afterRows))
	for _, row := range afterRows {
		afterEntries = append(afterEntries, TimelineEntry{
			ID: row.ID, SessionID: row.SessionID, Type: row.Type, Title: row.Title, Content: row.Content,
			ToolName: nullablePtr(row.ToolName), Project: nullablePtr(row.Project), Scope: row.Scope,
			TopicKey: nullablePtr(row.TopicKey), RevisionCount: int(row.RevisionCount),
			DuplicateCount: int(row.DuplicateCount), LastSeenAt: nullablePtr(row.LastSeenAt),
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: nullablePtr(row.DeletedAt),
		})
	}

	totalInRange, _ := s.q.CountSessionObservations(context.Background(), focus.SessionID)

	return &TimelineResult{
		Focus:        *focus,
		Before:       beforeEntries,
		After:        afterEntries,
		SessionInfo:  session,
		TotalInRange: int(totalInRange),
	}, nil
}

func (s *Store) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > s.cfg.MaxSearchResults {
		limit = s.cfg.MaxSearchResults
	}

	if strings.TrimSpace(query) == "" {
		return s.searchByFilter(opts, limit)
	}
	return s.searchFTS(query, opts, limit)
}

func (s *Store) searchByFilter(opts SearchOptions, limit int) ([]SearchResult, error) {
	tags := normalizeTagList(opts.Tags)
	rows, err := s.q.SearchObservationsByFilter(context.Background(), dbgen.SearchObservationsByFilterParams{
		PreferTagsJson: jsonStrings(normalizeTagList(opts.PreferTags)),
		Type:           opts.Type, Project: opts.Project, Scope: normalizeOptionalScope(opts.Scope),
		TopicKey: opts.TopicKey, TagCount: len(tags), TagsJson: jsonStrings(tags),
		ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, searchResultFromFilterRow(row))
	}
	if err := s.loadTagsForSearchResults(results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) searchFTS(query string, opts SearchOptions, limit int) ([]SearchResult, error) {
	ftsQuery := sanitizeFTS(query)
	tags := normalizeTagList(opts.Tags)
	rows, err := s.q.SearchObservationsFTS(context.Background(), dbgen.SearchObservationsFTSParams{
		PreferTagsJson: jsonStrings(normalizeTagList(opts.PreferTags)), FtsQuery: ftsQuery,
		Type: opts.Type, Project: opts.Project, Scope: normalizeOptionalScope(opts.Scope),
		TopicKey: opts.TopicKey, TagCount: len(tags), TagsJson: jsonStrings(tags),
		ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, searchResultFromFTSRow(row))
	}
	if err := s.loadTagsForSearchResults(results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) GetObservationBySyncID(syncID string) (*Observation, error) {
	row, err := s.q.GetLiveObservationBySyncID(context.Background(), sqlNullString(syncID))
	if err != nil {
		return nil, err
	}
	o := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
		row.ToolName, row.Project, row.Scope, row.TopicKey, row.RevisionCount, row.DuplicateCount,
		row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
	return &o, nil
}

func (s *Store) AddPrompt(p AddPromptParams) (int64, error) {
	content := stripPrivateTags(p.Content)
	if len(content) > s.cfg.MaxObservationLength {
		content = content[:s.cfg.MaxObservationLength] + "... [truncated]"
	}

	var promptID int64
	err := s.withTx(func(tx *sql.Tx) error {
		syncID := newSyncID("prompt")
		var err error
		promptID, err = s.q.WithTx(tx).InsertPrompt(context.Background(), dbgen.InsertPromptParams{
			SyncID: sqlNullString(syncID), SessionID: p.SessionID, Content: content, Project: sqlNullString(p.Project),
		})
		if err != nil {
			return err
		}
		return s.enqueueSyncMutationTx(tx, SyncEntityPrompt, syncID, SyncOpUpsert, syncPromptPayload{
			SyncID:    syncID,
			SessionID: p.SessionID,
			Content:   content,
			Project:   nullableString(p.Project),
		})
	})
	if err != nil {
		return 0, err
	}
	return promptID, nil
}

func (s *Store) RecentPrompts(project string, limit int) ([]Prompt, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.q.ListRecentPrompts(context.Background(), dbgen.ListRecentPromptsParams{
		Project: project, ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]Prompt, 0, len(rows))
	for _, row := range rows {
		results = append(results, Prompt{
			ID: row.ID, SyncID: dbString(row.SyncID), SessionID: row.SessionID,
			Content: row.Content, Project: dbString(row.Project), CreatedAt: row.CreatedAt,
		})
	}
	return results, nil
}

func (s *Store) SearchPrompts(query string, project string, limit int) ([]Prompt, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.q.SearchPromptsFTS(context.Background(), dbgen.SearchPromptsFTSParams{
		FtsQuery: sanitizeFTS(query), Project: project, ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search prompts: %w", err)
	}
	results := make([]Prompt, 0, len(rows))
	for _, row := range rows {
		results = append(results, Prompt{
			ID: row.ID, SyncID: dbString(row.SyncID), SessionID: row.SessionID,
			Content: row.Content, Project: dbString(row.Project), CreatedAt: row.CreatedAt,
		})
	}
	return results, nil
}

func normalizeTagList(raw []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, r := range raw {
		if n := normalizeTag(r); n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func (s *Store) getObservationTx(tx *sql.Tx, id int64) (*Observation, error) {
	row, err := s.q.WithTx(tx).GetLiveObservation(context.Background(), id)
	if err != nil {
		return nil, err
	}
	o := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
		row.ToolName, row.Project, row.Scope, row.TopicKey, row.RevisionCount, row.DuplicateCount,
		row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
	if err := s.loadTagsForObservationTx(tx, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) getObservationBySyncIDTx(tx *sql.Tx, syncID string, includeDeleted bool) (*Observation, error) {
	q := s.q.WithTx(tx)
	if includeDeleted {
		row, err := q.GetObservationBySyncIDIncludingDeleted(context.Background(), sqlNullString(syncID))
		if err != nil {
			return nil, err
		}
		o := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
			row.ToolName, row.Project, row.Scope, row.TopicKey, row.RevisionCount, row.DuplicateCount,
			row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
		return &o, nil
	}
	row, err := q.GetLiveObservationBySyncID(context.Background(), sqlNullString(syncID))
	if err != nil {
		return nil, err
	}
	o := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
		row.ToolName, row.Project, row.Scope, row.TopicKey, row.RevisionCount, row.DuplicateCount,
		row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
	return &o, nil
}
