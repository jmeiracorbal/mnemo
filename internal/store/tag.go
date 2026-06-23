package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

var tagAliases = map[string]string{
	"authentication": "auth",
	"authorization":  "auth",
	"database":       "db",
	"databases":      "db",
	"configuration":  "config",
	"configurations": "config",
	"deployment":     "deploy",
	"deployments":    "deploy",
	"testing":        "test",
	"tests":          "test",
	"errors":         "error",
	"panics":         "panic",
	"sessions":       "session",
	"memories":       "memory",
	"migrations":     "migration",
	"caching":        "cache",
	"caches":         "cache",
	"performances":   "performance",
	"securities":     "security",
	"refactoring":    "refactor",
	"syncing":        "sync",
	"bugs":           "bug",
	"fixes":          "bug",
	"fix":            "bug",
	"apis":           "api",
	"configs":        "config",
	"incidents":      "incident",
	"patterns":       "pattern",
	"decisions":      "decision",
}

var blockedTags = map[string]bool{
	"topic":   true,
	"general": true,
	"other":   true,
	"misc":    true,
	"stuff":   true,
	"thing":   true,
	"things":  true,
	"item":    true,
	"items":   true,
	"note":    true,
	"notes":   true,
	"update":  true,
	"change":  true,
}

const (
	maxTagsPerObservation = 10
	maxTagsPerSession     = 5
)

func isBlockedTag(tag string) bool {
	return blockedTags[tag]
}

func normalizeTagBase(s string) string {
	v := strings.TrimSpace(strings.ToLower(s))
	if v == "" {
		return ""
	}
	v = nonAlnumRe.ReplaceAllString(v, "-")
	v = strings.Trim(v, "-")
	if len(v) < 2 {
		return ""
	}
	if len(v) > 50 {
		v = v[:50]
	}
	return v
}

func normalizeTag(s string) string {
	v := normalizeTagBase(s)
	if v == "" {
		return ""
	}
	if canonical, ok := tagAliases[v]; ok {
		return canonical
	}
	return v
}

// NormalizeTag is the exported equivalent of normalizeTag.
func NormalizeTag(s string) string { return normalizeTag(s) }

func (s *Store) ListTags(project string) ([]TagInfo, error) {
	rows, err := s.q.ListTagAggregates(context.Background(), project)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	tags := make([]TagInfo, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, TagInfo{Tag: row.Tag, Count: int(row.Count), LastUsedAt: dbString(row.LastUsedAt)})
	}
	return tags, nil
}

func (s *Store) TagStats(project string, opts TagStatsOptions) ([]TagInfo, error) {
	rows, err := s.q.ListTagAggregates(context.Background(), project)
	if err != nil {
		return nil, fmt.Errorf("tag stats: %w", err)
	}
	tags := make([]TagInfo, 0, len(rows))
	for _, row := range rows {
		tag := TagInfo{Tag: row.Tag, Count: int(row.Count), LastUsedAt: dbString(row.LastUsedAt)}
		if opts.MinCount > 0 && tag.Count < opts.MinCount {
			continue
		}
		if opts.MaxCount > 0 && tag.Count > opts.MaxCount {
			continue
		}
		if !opts.UnusedSince.IsZero() {
			usedAt, parseErr := parseTagTimestamp(tag.LastUsedAt)
			if parseErr != nil || !usedAt.Before(opts.UnusedSince.UTC()) {
				continue
			}
		}
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		switch opts.SortBy {
		case "stale":
			if tags[i].LastUsedAt != tags[j].LastUsedAt {
				return tags[i].LastUsedAt < tags[j].LastUsedAt
			}
		case "alpha":
			return tags[i].Tag < tags[j].Tag
		default:
			if tags[i].Count != tags[j].Count {
				return tags[i].Count > tags[j].Count
			}
		}
		return tags[i].Tag < tags[j].Tag
	})
	if opts.Limit > 0 && len(tags) > opts.Limit {
		tags = tags[:opts.Limit]
	}
	return tags, nil
}

func (s *Store) RelatedTags(project, tag string, opts RelatedTagsOptions) ([]RelatedTag, error) {
	tag = NormalizeTag(tag)
	if tag == "" {
		return nil, fmt.Errorf("tag must not be empty")
	}

	includeObs := opts.IncludeObservations || (!opts.IncludeObservations && !opts.IncludeSessions)
	includeSes := opts.IncludeSessions || (!opts.IncludeObservations && !opts.IncludeSessions)

	type entry struct {
		count    int
		lastSeen string
	}
	acc := make(map[string]*entry)

	maxDate := func(a, b string) string {
		if a > b {
			return a
		}
		return b
	}

	if includeObs {
		since := ""
		if !opts.Since.IsZero() {
			since = opts.Since.UTC().Format(time.RFC3339)
		}
		rows, err := s.q.ListRelatedObservationTags(context.Background(), dbgen.ListRelatedObservationTagsParams{
			Tag: tag, Project: project, Since: since,
		})
		if err != nil {
			return nil, fmt.Errorf("related tags (observations): %w", err)
		}
		for _, row := range rows {
			last := dbString(row.LastSeenAt)
			if e, ok := acc[row.Tag]; ok {
				e.count += int(row.Count)
				e.lastSeen = maxDate(e.lastSeen, last)
			} else {
				acc[row.Tag] = &entry{count: int(row.Count), lastSeen: last}
			}
		}
	}

	if includeSes {
		since := ""
		if !opts.Since.IsZero() {
			since = opts.Since.UTC().Format(time.RFC3339)
		}
		rows, err := s.q.ListRelatedSessionTags(context.Background(), dbgen.ListRelatedSessionTagsParams{
			Tag: tag, Project: project, Since: since,
		})
		if err != nil {
			return nil, fmt.Errorf("related tags (sessions): %w", err)
		}
		for _, row := range rows {
			last := dbString(row.LastSeenAt)
			if e, ok := acc[row.Tag]; ok {
				e.count += int(row.Count)
				e.lastSeen = maxDate(e.lastSeen, last)
			} else {
				acc[row.Tag] = &entry{count: int(row.Count), lastSeen: last}
			}
		}
	}

	minC := opts.MinCooccurrence
	if minC <= 0 {
		minC = 1
	}
	var result []RelatedTag
	for t, e := range acc {
		if e.count < minC {
			continue
		}
		result = append(result, RelatedTag{
			Tag:               t,
			CooccurrenceCount: e.count,
			Score:             float64(e.count),
			LastSeenAt:        e.lastSeen,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].Tag < result[j].Tag
	})

	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}
	return result, nil
}

func (s *Store) MergeTags(fromTag, toTag string) (obsCount int, sessCount int, err error) {
	from := normalizeTagBase(fromTag)
	to := normalizeTag(toTag)
	if from == "" || to == "" {
		return 0, 0, fmt.Errorf("both from and to tags must be non-empty after normalization")
	}
	if from == to {
		return 0, 0, fmt.Errorf("from and to tags are identical after normalization: %q", to)
	}
	if isBlockedTag(to) {
		return 0, 0, fmt.Errorf("target tag %q is blocked (too generic)", to)
	}

	err = s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		obsRows, err := q.ListObservationsAffectedByTag(context.Background(), from)
		if err != nil {
			return fmt.Errorf("merge: collect observations: %w", err)
		}
		type affectedObs struct {
			id      int64
			payload syncObservationPayload
		}
		obsList := make([]affectedObs, 0, len(obsRows))
		for _, row := range obsRows {
			obsList = append(obsList, affectedObs{
				id: row.ID,
				payload: syncObservationPayload{
					SyncID: dbString(row.SyncID), SessionID: row.SessionID, Type: row.Type,
					Title: row.Title, Content: row.Content, ToolName: nullablePtr(row.ToolName),
					Project: nullablePtr(row.Project), Scope: row.Scope, TopicKey: nullablePtr(row.TopicKey),
				},
			})
		}

		sessRows, err := q.ListSessionsAffectedByTag(context.Background(), from)
		if err != nil {
			return fmt.Errorf("merge: collect sessions: %w", err)
		}
		type affectedSess struct {
			payload syncSessionPayload
		}
		sessList := make([]affectedSess, 0, len(sessRows))
		for _, row := range sessRows {
			sessList = append(sessList, affectedSess{payload: syncSessionPayload{
				ID: row.ID, Project: row.Project, Directory: row.Directory,
				EndedAt: nullablePtr(row.EndedAt), Summary: nullablePtr(row.Summary),
			}})
		}

		if err := q.CopyObservationTag(context.Background(), dbgen.CopyObservationTagParams{
			ToTag: to, FromTag: from,
		}); err != nil {
			return fmt.Errorf("merge observation_tags insert: %w", err)
		}
		if err := q.DeleteObservationTagByName(context.Background(), from); err != nil {
			return fmt.Errorf("merge observation_tags delete: %w", err)
		}
		obsCount = len(obsList)

		if err := q.CopySessionTag(context.Background(), dbgen.CopySessionTagParams{
			ToTag: to, FromTag: from,
		}); err != nil {
			return fmt.Errorf("merge session_tags insert: %w", err)
		}
		if err := q.DeleteSessionTagByName(context.Background(), from); err != nil {
			return fmt.Errorf("merge session_tags delete: %w", err)
		}
		sessCount = len(sessList)

		for _, r := range obsList {
			var o Observation
			o.ID = r.id
			if err := s.loadTagsForObservationTx(tx, &o); err != nil {
				return fmt.Errorf("merge: load observation tags: %w", err)
			}
			tags := o.Tags
			if tags == nil {
				tags = []string{}
			}
			r.payload.Tags = &tags
			if err := s.enqueueSyncMutationTx(tx, SyncEntityObservation, r.payload.SyncID, SyncOpUpsert, r.payload); err != nil {
				return fmt.Errorf("merge: enqueue observation sync: %w", err)
			}
		}

		for _, r := range sessList {
			var sess Session
			sess.ID = r.payload.ID
			if err := s.loadTagsForSessionTx(tx, &sess); err != nil {
				return fmt.Errorf("merge: load session tags: %w", err)
			}
			tags := sess.Tags
			if tags == nil {
				tags = []string{}
			}
			r.payload.Tags = &tags
			if err := s.enqueueSyncMutationTx(tx, SyncEntitySession, r.payload.ID, SyncOpUpsert, r.payload); err != nil {
				return fmt.Errorf("merge: enqueue session sync: %w", err)
			}
		}

		return nil
	})
	return obsCount, sessCount, err
}

func (s *Store) SetSessionTags(id string, tags []string) error {
	return s.withTx(func(tx *sql.Tx) error {
		if err := s.setTagsForSessionTx(tx, id, tags); err != nil {
			return err
		}
		row, err := s.q.WithTx(tx).GetSessionPayload(context.Background(), id)
		if err != nil {
			return err
		}
		var sess Session
		sess.ID = id
		if err := s.loadTagsForSessionTx(tx, &sess); err != nil {
			return err
		}
		storedTags := sess.Tags
		if storedTags == nil {
			storedTags = []string{}
		}
		return s.enqueueSyncMutationTx(tx, SyncEntitySession, id, SyncOpUpsert, syncSessionPayload{
			ID:        id,
			Project:   row.Project,
			Directory: row.Directory,
			EndedAt:   nullablePtr(row.EndedAt),
			Summary:   nullablePtr(row.Summary),
			Tags:      &storedTags,
		})
	})
}

func (s *Store) setTagsForObservationTx(tx *sql.Tx, obsID int64, tags []string) error {
	q := s.q.WithTx(tx)
	if err := q.DeleteObservationTags(context.Background(), obsID); err != nil {
		return err
	}
	seen := make(map[string]bool)
	count := 0
	for _, tag := range tags {
		norm := normalizeTag(tag)
		if norm == "" || isBlockedTag(norm) || seen[norm] {
			continue
		}
		if count >= maxTagsPerObservation {
			break
		}
		seen[norm] = true
		count++
		if err := q.InsertObservationTag(context.Background(), dbgen.InsertObservationTagParams{
			ObservationID: obsID, Tag: norm,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadTagsForObservations(obs []Observation) error {
	if len(obs) == 0 {
		return nil
	}
	ids := make([]int64, len(obs))
	for i := range obs {
		ids[i] = obs[i].ID
	}
	rows, err := s.q.ListTagsForObservationIDs(context.Background(), ids)
	if err != nil {
		return fmt.Errorf("load tags for observations: %w", err)
	}
	tagMap := make(map[int64][]string)
	for _, row := range rows {
		tagMap[row.ObservationID] = append(tagMap[row.ObservationID], row.Tag)
	}
	for i := range obs {
		if tags, ok := tagMap[obs[i].ID]; ok {
			obs[i].Tags = tags
		}
	}
	return nil
}

func (s *Store) loadTagsForObservationTx(tx *sql.Tx, o *Observation) error {
	rows, err := s.q.WithTx(tx).ListObservationTags(context.Background(), o.ID)
	if err != nil {
		return fmt.Errorf("load observation tags: %w", err)
	}
	o.Tags = append(o.Tags, rows...)
	return nil
}

func (s *Store) loadTagsForObservation(o *Observation) error {
	rows, err := s.q.ListObservationTags(context.Background(), o.ID)
	if err != nil {
		return fmt.Errorf("load observation tags: %w", err)
	}
	o.Tags = append(o.Tags, rows...)
	return nil
}

func (s *Store) setTagsForSessionTx(tx *sql.Tx, sessionID string, tags []string) error {
	q := s.q.WithTx(tx)
	if err := q.DeleteSessionTags(context.Background(), sessionID); err != nil {
		return err
	}
	seen := make(map[string]bool)
	count := 0
	for _, raw := range tags {
		tag := normalizeTag(raw)
		if tag == "" || isBlockedTag(tag) || seen[tag] {
			continue
		}
		if count >= maxTagsPerSession {
			break
		}
		seen[tag] = true
		count++
		if err := q.InsertSessionTag(context.Background(), dbgen.InsertSessionTagParams{
			SessionID: sessionID, Tag: tag,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadTagsForSession(sess *Session) error {
	rows, err := s.q.ListSessionTags(context.Background(), sess.ID)
	if err != nil {
		return fmt.Errorf("load session tags: %w", err)
	}
	sess.Tags = append(sess.Tags, rows...)
	return nil
}

func (s *Store) loadTagsForSessionTx(tx *sql.Tx, sess *Session) error {
	rows, err := s.q.WithTx(tx).ListSessionTags(context.Background(), sess.ID)
	if err != nil {
		return fmt.Errorf("load session tags: %w", err)
	}
	sess.Tags = append(sess.Tags, rows...)
	return nil
}

func (s *Store) loadTagsForSearchResults(results []SearchResult) error {
	if len(results) == 0 {
		return nil
	}
	ids := make([]int64, len(results))
	for i := range results {
		ids[i] = results[i].ID
	}
	rows, err := s.q.ListTagsForObservationIDs(context.Background(), ids)
	if err != nil {
		return fmt.Errorf("load tags for search results: %w", err)
	}
	tagMap := make(map[int64][]string)
	for _, row := range rows {
		tagMap[row.ObservationID] = append(tagMap[row.ObservationID], row.Tag)
	}
	for i := range results {
		if tags, ok := tagMap[results[i].ID]; ok {
			results[i].Tags = tags
		}
	}
	return nil
}

func parseTagTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC)
}

func tagWeights(tags []TagInfo, topN int) []TagWeight {
	if len(tags) == 0 {
		return nil
	}

	n := topN
	if n < 0 {
		n = 0
	}

	sorted := make([]TagInfo, len(tags))
	copy(sorted, tags)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
	dominant := make(map[string]bool, n)
	for i, ti := range sorted {
		if i >= n {
			break
		}
		dominant[ti.Tag] = true
	}

	var oldest, newest time.Time
	for _, ti := range tags {
		t, err := parseTagTimestamp(ti.LastUsedAt)
		if err != nil {
			continue
		}
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
		if newest.IsZero() || t.After(newest) {
			newest = t
		}
	}
	span := newest.Sub(oldest).Seconds()

	out := make([]TagWeight, len(tags))
	for i, ti := range tags {
		w := TagWeight{
			Tag:        ti.Tag,
			Frequency:  ti.Count,
			IsDominant: dominant[ti.Tag],
		}
		t, err := parseTagTimestamp(ti.LastUsedAt)
		if err == nil {
			if span > 0 {
				w.RecencyScore = t.Sub(oldest).Seconds() / span
			} else {
				w.RecencyScore = 1.0
			}
		}
		out[i] = w
	}
	return out
}
