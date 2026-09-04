package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

const (
	MemoryReviewStateNeedsReview = "needs-review"
	MemoryReviewStateReviewed    = "reviewed"
	MemoryReviewStateStale       = "stale"
	MemoryReviewStateSuperseded  = "superseded"
)

type MemoryReviewOptions struct {
	Project  string
	Scope    string
	TopicKey string
	Limit    int
}

type MemoryReviewInfo struct {
	State        string `json:"state,omitempty"`
	Reason       string `json:"reason,omitempty"`
	SupersededBy *int64 `json:"superseded_by,omitempty"`
	ReviewedAt   string `json:"reviewed_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type MemoryConflictObservation struct {
	ID          int64             `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Project     string            `json:"project,omitempty"`
	Scope       string            `json:"scope"`
	TopicKey    string            `json:"topic_key,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ReviewState *MemoryReviewInfo `json:"review_state,omitempty"`
	Provenance  *Provenance       `json:"provenance,omitempty"`
}

type MemoryConflictGroup struct {
	ID              string                      `json:"id"`
	Kind            string                      `json:"kind"`
	Reason          string                      `json:"reason"`
	Confidence      float64                     `json:"confidence"`
	Project         string                      `json:"project,omitempty"`
	Scope           string                      `json:"scope,omitempty"`
	TopicKey        string                      `json:"topic_key,omitempty"`
	SuggestedAction string                      `json:"suggested_action"`
	Observations    []MemoryConflictObservation `json:"observations"`
}

type MemoryReviewReport struct {
	Total  int                   `json:"total"`
	Groups []MemoryConflictGroup `json:"groups"`
}

type MemoryTopicConsolidationPlan struct {
	FromTopic string  `json:"from_topic"`
	ToTopic   string  `json:"to_topic"`
	Project   string  `json:"project,omitempty"`
	Scope     string  `json:"scope,omitempty"`
	Total     int     `json:"total"`
	IDs       []int64 `json:"ids"`
}

func (s *Store) ReviewMemoryConflicts(opts MemoryReviewOptions) (*MemoryReviewReport, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	candidates, err := s.listMemoryReviewCandidates(opts)
	if err != nil {
		return nil, err
	}
	groups := buildMemoryConflictGroups(candidates, opts.Limit)
	return &MemoryReviewReport{Total: len(groups), Groups: groups}, nil
}

func (s *Store) MarkMemoryReviewed(id int64, reason string) error {
	return s.setMemoryReviewState(id, MemoryReviewStateReviewed, reason, nil)
}

func (s *Store) MarkMemoryStale(id int64, reason string) error {
	return s.setMemoryReviewState(id, MemoryReviewStateStale, reason, nil)
}

func (s *Store) SupersedeMemory(oldID, newID int64, reason string) error {
	if oldID == newID {
		return fmt.Errorf("cannot supersede an observation by itself")
	}
	if _, err := s.GetObservation(newID); err != nil {
		return fmt.Errorf("superseding observation %d: %w", newID, err)
	}
	return s.setMemoryReviewState(oldID, MemoryReviewStateSuperseded, reason, &newID)
}

func (s *Store) PlanMemoryTopicConsolidation(fromTopic, toTopic, project, scope string) (*MemoryTopicConsolidationPlan, error) {
	fromTopic = normalizeTopicKey(fromTopic)
	toTopic = normalizeTopicKey(toTopic)
	if fromTopic == "" {
		return nil, fmt.Errorf("from topic must not be empty")
	}
	if toTopic == "" {
		return nil, fmt.Errorf("to topic must not be empty")
	}
	if fromTopic == toTopic {
		return nil, fmt.Errorf("from and to topics must differ")
	}
	ids, err := s.listObservationIDsByTopic(fromTopic, project, scope)
	if err != nil {
		return nil, err
	}
	return &MemoryTopicConsolidationPlan{
		FromTopic: fromTopic,
		ToTopic:   toTopic,
		Project:   strings.TrimSpace(project),
		Scope:     normalizeOptionalScope(scope),
		Total:     len(ids),
		IDs:       ids,
	}, nil
}

func (s *Store) ConsolidateMemoryTopic(fromTopic, toTopic, project, scope string) (*MemoryTopicConsolidationPlan, error) {
	plan, err := s.PlanMemoryTopicConsolidation(fromTopic, toTopic, project, scope)
	if err != nil {
		return nil, err
	}
	for _, id := range plan.IDs {
		newTopic := plan.ToTopic
		if _, err := s.UpdateObservation(id, UpdateObservationParams{TopicKey: &newTopic}); err != nil {
			return plan, fmt.Errorf("update observation %d topic: %w", id, err)
		}
	}
	return plan, nil
}

type memoryReviewCandidate struct {
	MemoryConflictObservation
	normalizedHash  string
	normalizedTitle string
}

func (s *Store) listMemoryReviewCandidates(opts MemoryReviewOptions) ([]memoryReviewCandidate, error) {
	project := strings.TrimSpace(opts.Project)
	scope := normalizeOptionalScope(opts.Scope)
	topic := normalizeTopicKey(opts.TopicKey)
	rows, err := s.q.ListMemoryReviewCandidates(context.Background(), dbgen.ListMemoryReviewCandidatesParams{
		Project: project, Scope: scope, TopicKey: topic,
	})
	if err != nil {
		return nil, fmt.Errorf("review memory conflicts: %w", err)
	}

	out := make([]memoryReviewCandidate, 0, len(rows))
	for _, row := range rows {
		state := dbString(row.ReviewState)
		c := memoryReviewCandidate{
			MemoryConflictObservation: MemoryConflictObservation{
				ID:        row.ID,
				Type:      row.Type,
				Title:     row.Title,
				Project:   dbString(row.Project),
				Scope:     row.Scope,
				TopicKey:  dbString(row.TopicKey),
				CreatedAt: row.CreatedAt,
				UpdatedAt: row.UpdatedAt,
			},
			normalizedHash: dbString(row.NormalizedHash),
		}
		if row.ProvenanceID.Valid {
			provenance, err := s.getProvenance(row.ProvenanceID.Int64)
			if err != nil {
				return nil, fmt.Errorf("review memory conflicts: provenance: %w", err)
			}
			c.Provenance = provenance
		}
		c.normalizedTitle = normalizeConflictText(c.Title)
		if state != "" {
			info := &MemoryReviewInfo{
				State:      state,
				Reason:     dbString(row.ReviewReason),
				ReviewedAt: dbString(row.ReviewReviewedAt),
				UpdatedAt:  dbString(row.ReviewUpdatedAt),
			}
			if row.SupersededBy.Valid {
				supersededBy := row.SupersededBy.Int64
				info.SupersededBy = &supersededBy
			}
			c.ReviewState = info
		}
		if isResolvedMemoryReviewState(state) {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) setMemoryReviewState(id int64, state, reason string, supersededBy *int64) error {
	if !validMemoryReviewState(state) {
		return fmt.Errorf("unsupported review state %q", state)
	}
	if _, err := s.GetObservation(id); err != nil {
		return fmt.Errorf("observation %d: %w", id, err)
	}
	reviewedAt := sql.NullString{}
	if state == MemoryReviewStateReviewed {
		reviewedAt = sqlNullString(Now())
	}
	params := dbgen.UpsertMemoryReviewStateParams{
		ObservationID: id,
		State:         state,
		Reason:        strings.TrimSpace(reason),
		SupersededBy:  sqlNullInt64Ptr(supersededBy),
		ReviewedAt:    reviewedAt,
	}
	return s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.UpsertMemoryReviewState(context.Background(), params); err != nil {
			return fmt.Errorf("set memory review state: %w", err)
		}
		return s.backfillCanonicalTableTx(tx, "observation_reviews")
	})
}

func (s *Store) listObservationIDsByTopic(topic, project, scope string) ([]int64, error) {
	project = strings.TrimSpace(project)
	scope = normalizeOptionalScope(scope)
	ids, err := s.q.ListObservationIDsByTopic(context.Background(), dbgen.ListObservationIDsByTopicParams{
		TopicKey: sqlNullString(topic), Project: project, Scope: scope,
	})
	if err != nil {
		return nil, fmt.Errorf("list observations by topic: %w", err)
	}
	return ids, nil
}

func sqlNullInt64Ptr(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func buildMemoryConflictGroups(candidates []memoryReviewCandidate, limit int) []MemoryConflictGroup {
	var groups []MemoryConflictGroup
	seen := map[string]bool{}
	addGroups := func(kind string, byKey map[string][]memoryReviewCandidate) {
		keys := make([]string, 0, len(byKey))
		for key := range byKey {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			items := byKey[key]
			if len(items) < 2 {
				continue
			}
			sig := conflictSignature(items)
			if seen[sig] {
				continue
			}
			seen[sig] = true
			groups = append(groups, newMemoryConflictGroup(kind, key, items))
		}
	}

	byTopic := map[string][]memoryReviewCandidate{}
	byHash := map[string][]memoryReviewCandidate{}
	byTitle := map[string][]memoryReviewCandidate{}
	for _, c := range candidates {
		baseKey := c.Project + "\x00" + c.Scope + "\x00"
		if c.TopicKey != "" {
			byTopic[baseKey+c.TopicKey] = append(byTopic[baseKey+c.TopicKey], c)
		}
		if c.normalizedHash != "" {
			byHash[baseKey+c.normalizedHash] = append(byHash[baseKey+c.normalizedHash], c)
		}
		if c.normalizedTitle != "" {
			byTitle[baseKey+c.normalizedTitle] = append(byTitle[baseKey+c.normalizedTitle], c)
		}
	}
	addGroups("topic-conflict", byTopic)
	addGroups("duplicate-content", byHash)
	addGroups("duplicate-title", byTitle)

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Confidence != groups[j].Confidence {
			return groups[i].Confidence > groups[j].Confidence
		}
		return groups[i].ID < groups[j].ID
	})
	if limit > 0 && len(groups) > limit {
		groups = groups[:limit]
	}
	return groups
}

func newMemoryConflictGroup(kind, key string, items []memoryReviewCandidate) MemoryConflictGroup {
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt > items[j].UpdatedAt })
	observations := make([]MemoryConflictObservation, 0, len(items))
	for _, item := range items {
		observations = append(observations, item.MemoryConflictObservation)
	}
	group := MemoryConflictGroup{
		ID:           kind + ":" + strings.ReplaceAll(key, "\x00", ":"),
		Kind:         kind,
		Project:      items[0].Project,
		Scope:        items[0].Scope,
		TopicKey:     items[0].TopicKey,
		Observations: observations,
	}
	switch kind {
	case "duplicate-content":
		group.Confidence = 0.95
		group.Reason = "multiple live memories have identical normalized content"
		group.SuggestedAction = "choose a canonical memory and mark duplicates as superseded or stale"
	case "topic-conflict":
		group.Confidence = 0.85
		group.Reason = "multiple live memories share the same topic key"
		group.SuggestedAction = "pick the current canonical memory and supersede older conflicting memories"
	case "duplicate-title":
		group.Confidence = 0.65
		group.Reason = "multiple live memories share the same normalized title"
		group.SuggestedAction = "review whether these memories should share a topic key or supersede each other"
	}
	return group
}

func conflictSignature(items []memoryReviewCandidate) string {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprint(id))
	}
	return strings.Join(parts, ",")
}

func normalizeConflictText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func validMemoryReviewState(state string) bool {
	switch state {
	case MemoryReviewStateNeedsReview, MemoryReviewStateReviewed, MemoryReviewStateStale, MemoryReviewStateSuperseded:
		return true
	default:
		return false
	}
}

func isResolvedMemoryReviewState(state string) bool {
	switch state {
	case MemoryReviewStateReviewed, MemoryReviewStateStale, MemoryReviewStateSuperseded:
		return true
	default:
		return false
	}
}
