package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func mustDefaultConfig(t *testing.T) Config {
	t.Helper()
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig: %v", err)
	}
	return cfg
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	cfg := mustDefaultConfig(t)
	cfg.DataDir = t.TempDir()
	cfg.DedupeWindow = time.Hour

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

type fakeRows struct {
	next    []bool
	scanErr error
	err     error
}

func (f *fakeRows) Next() bool {
	if len(f.next) == 0 {
		return false
	}
	v := f.next[0]
	f.next = f.next[1:]
	return v
}

func (f *fakeRows) Scan(dest ...any) error {
	return f.scanErr
}

func (f *fakeRows) Err() error {
	return f.err
}

func (f *fakeRows) Close() error {
	return nil
}

func TestAddObservationDeduplicatesWithinWindow(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	firstID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "Fixed tokenizer",
		Content:   "Normalized tokenizer panic on edge case",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add first observation: %v", err)
	}

	secondID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "Fixed tokenizer",
		Content:   "normalized   tokenizer panic on EDGE case",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add duplicate observation: %v", err)
	}

	if firstID != secondID {
		t.Fatalf("expected duplicate to reuse same id, got %d and %d", firstID, secondID)
	}

	obs, err := s.GetObservation(firstID)
	if err != nil {
		t.Fatalf("get deduped observation: %v", err)
	}
	if obs.DuplicateCount != 2 {
		t.Fatalf("expected duplicate_count=2, got %d", obs.DuplicateCount)
	}
}

func TestScopeFiltersSearchAndContext(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "decision",
		Title:     "Project auth",
		Content:   "Keep auth middleware in project memory",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add project observation: %v", err)
	}

	_, err = s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "decision",
		Title:     "Personal note",
		Content:   "Use this regex trick later",
		Project:   "mnemo",
		Scope:     "personal",
	})
	if err != nil {
		t.Fatalf("add personal observation: %v", err)
	}

	projectResults, err := s.Search("regex", SearchOptions{Project: "mnemo", Scope: "project", Limit: 10})
	if err != nil {
		t.Fatalf("search project scope: %v", err)
	}
	if len(projectResults) != 0 {
		t.Fatalf("expected no project-scope regex results, got %d", len(projectResults))
	}

	personalResults, err := s.Search("regex", SearchOptions{Project: "mnemo", Scope: "personal", Limit: 10})
	if err != nil {
		t.Fatalf("search personal scope: %v", err)
	}
	if len(personalResults) != 1 {
		t.Fatalf("expected 1 personal-scope result, got %d", len(personalResults))
	}

	ctx, err := s.FormatContext("mnemo", "personal")
	if err != nil {
		t.Fatalf("format context personal: %v", err)
	}
	if !strings.Contains(ctx, "Personal note") {
		t.Fatalf("expected personal context to include personal observation")
	}
	if strings.Contains(ctx, "Project auth") {
		t.Fatalf("expected personal context to exclude project observation")
	}
}

func TestUpdateAndSoftDeleteExcludedFromSearchAndTimeline(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	firstID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "first",
		Content:   "first event",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}

	middleID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "middle",
		Content:   "to be deleted",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add middle: %v", err)
	}

	lastID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "last",
		Content:   "last event",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add last: %v", err)
	}

	newTitle := "last-updated"
	newContent := "updated content"
	newScope := "personal"
	updated, err := s.UpdateObservation(lastID, UpdateObservationParams{
		Title:   &newTitle,
		Content: &newContent,
		Scope:   &newScope,
	})
	if err != nil {
		t.Fatalf("update observation: %v", err)
	}
	if updated.Title != newTitle || updated.Scope != "personal" {
		t.Fatalf("update did not apply; got title=%q scope=%q", updated.Title, updated.Scope)
	}

	if err := s.DeleteObservation(middleID, false); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if _, err := s.GetObservation(middleID); err == nil {
		t.Fatalf("expected deleted observation to be hidden from GetObservation")
	}

	searchResults, err := s.Search("deleted", SearchOptions{Project: "mnemo", Limit: 10})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(searchResults) != 0 {
		t.Fatalf("expected deleted observation excluded from search")
	}

	timeline, err := s.Timeline(firstID, 5, 5)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if len(timeline.After) != 1 || timeline.After[0].ID != lastID {
		t.Fatalf("expected timeline to skip deleted observation")
	}

	if err := s.DeleteObservation(lastID, true); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	if _, err := s.GetObservation(lastID); err == nil {
		t.Fatalf("expected hard-deleted observation to be missing")
	}
}

func TestTopicKeyUpsertUpdatesSameTopicWithoutCreatingNewRow(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	firstID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "architecture",
		Title:     "Auth architecture",
		Content:   "Use middleware for JWT validation.",
		Project:   "mnemo",
		Scope:     "project",
		TopicKey:  "architecture auth model",
	})
	if err != nil {
		t.Fatalf("add first architecture: %v", err)
	}

	secondID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "architecture",
		Title:     "Auth architecture",
		Content:   "Move auth to gateway + middleware chain.",
		Project:   "mnemo",
		Scope:     "project",
		TopicKey:  "ARCHITECTURE   AUTH  MODEL",
	})
	if err != nil {
		t.Fatalf("upsert architecture: %v", err)
	}

	if firstID != secondID {
		t.Fatalf("expected topic upsert to reuse id, got %d and %d", firstID, secondID)
	}

	obs, err := s.GetObservation(firstID)
	if err != nil {
		t.Fatalf("get upserted observation: %v", err)
	}
	if obs.RevisionCount != 2 {
		t.Fatalf("expected revision_count=2, got %d", obs.RevisionCount)
	}
	if obs.TopicKey == nil || *obs.TopicKey != "architecture-auth-model" {
		t.Fatalf("expected normalized topic key, got %v", obs.TopicKey)
	}
	if !strings.Contains(obs.Content, "gateway") {
		t.Fatalf("expected latest content after upsert, got %q", obs.Content)
	}
}

func TestDifferentTopicsDoNotReplaceEachOther(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	archID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "architecture",
		Title:     "Auth architecture",
		Content:   "Architecture decision",
		Project:   "mnemo",
		Scope:     "project",
		TopicKey:  "architecture/auth",
	})
	if err != nil {
		t.Fatalf("add architecture observation: %v", err)
	}

	bugID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "Fix auth nil panic",
		Content:   "Bugfix details",
		Project:   "mnemo",
		Scope:     "project",
		TopicKey:  "bug/auth-nil-panic",
	})
	if err != nil {
		t.Fatalf("add bug observation: %v", err)
	}

	if archID == bugID {
		t.Fatalf("expected different topic keys to create different observations")
	}

	observations, err := s.AllObservations("mnemo", "project", 10)
	if err != nil {
		t.Fatalf("all observations: %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(observations))
	}
}

func TestNewMigratesLegacyObservationIDSchema(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			directory TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			summary TEXT
		);
		CREATE TABLE observations (
			id INT,
			session_id TEXT,
			type TEXT,
			title TEXT,
			content TEXT,
			tool_name TEXT,
			project TEXT,
			created_at TEXT
		);
		INSERT INTO sessions (id, project, directory) VALUES ('s1', 'mnemo', '/tmp/mnemo');
		INSERT INTO observations (id, session_id, type, title, content, project, created_at)
		VALUES
			(NULL, 's1', 'bugfix', 'legacy null', 'legacy null content', 'mnemo', datetime('now')),
			(7, 's1', 'bugfix', 'legacy fixed', 'legacy fixed content', 'mnemo', datetime('now')),
			(7, 's1', 'bugfix', 'legacy duplicate', 'legacy duplicate content', 'mnemo', datetime('now'));
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("seed legacy db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	cfg := mustDefaultConfig(t)
	cfg.DataDir = dataDir

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("new store after legacy schema: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	obs, err := s.AllObservations("mnemo", "", 20)
	if err != nil {
		t.Fatalf("all observations after migration: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("expected 3 migrated observations, got %d", len(obs))
	}

	seen := make(map[int64]bool)
	for _, o := range obs {
		if o.ID <= 0 {
			t.Fatalf("expected migrated observation id > 0, got %d", o.ID)
		}
		if seen[o.ID] {
			t.Fatalf("expected unique migrated ids, duplicate %d", o.ID)
		}
		seen[o.ID] = true
	}

	results, err := s.Search("legacy", SearchOptions{Project: "mnemo", Limit: 10})
	if err != nil {
		t.Fatalf("search after migration: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected search results after migration")
	}

	newID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "bugfix",
		Title:     "post migration",
		Content:   "new row should get id",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add observation after migration: %v", err)
	}
	if newID <= 0 {
		t.Fatalf("expected autoincrement id after migration, got %d", newID)
	}
}

func TestNewMigratesLegacyUserPromptsSyncIDSchema(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			directory TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			summary TEXT
		);
		CREATE TABLE user_prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			content TEXT NOT NULL,
			project TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
		INSERT INTO sessions (id, project, directory) VALUES ('s1', 'mnemo', '/tmp/mnemo');
		INSERT INTO user_prompts (session_id, content, project) VALUES ('s1', 'legacy prompt', 'mnemo');
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("seed legacy db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	cfg := mustDefaultConfig(t)
	cfg.DataDir = dataDir

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("new store after legacy prompt schema: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var syncID string
	if err := s.db.QueryRow("SELECT sync_id FROM user_prompts WHERE content = ?", "legacy prompt").Scan(&syncID); err != nil {
		t.Fatalf("query migrated prompt sync_id: %v", err)
	}
	if syncID == "" {
		t.Fatalf("expected migrated prompt sync_id to be backfilled")
	}

	var hasSyncIDColumn bool
	rows, err := s.db.Query("PRAGMA table_info(user_prompts)")
	if err != nil {
		t.Fatalf("query prompt columns: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan prompt column: %v", err)
		}
		if name == "sync_id" {
			hasSyncIDColumn = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate prompt columns: %v", err)
	}
	if !hasSyncIDColumn {
		t.Fatalf("expected user_prompts.sync_id column after migration")
	}

	var indexName string
	if err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_prompts_sync_id'").Scan(&indexName); err != nil {
		t.Fatalf("query prompt sync index: %v", err)
	}
	if indexName != "idx_prompts_sync_id" {
		t.Fatalf("expected idx_prompts_sync_id to exist, got %q", indexName)
	}
}

func TestSuggestTopicKeyNormalizesDeterministically(t *testing.T) {
	got := SuggestTopicKey("Architecture", "  Auth Model  ", "ignored")
	if got != "architecture/auth-model" {
		t.Fatalf("expected architecture/auth-model, got %q", got)
	}

	fallback := SuggestTopicKey("bugfix", "", "Fix nil panic in auth middleware on empty token")
	if fallback != "bug/fix-nil-panic-in-auth-middleware-on-empty" {
		t.Fatalf("unexpected fallback topic key: %q", fallback)
	}
}

func TestSuggestTopicKeyInfersFamilyFromTextWhenTypeIsGeneric(t *testing.T) {
	bug := SuggestTopicKey("manual", "", "Fix regression in auth login flow")
	if bug != "bug/fix-regression-in-auth-login-flow" {
		t.Fatalf("expected bug family inference, got %q", bug)
	}

	arch := SuggestTopicKey("", "ADR: Split API gateway boundary", "")
	if arch != "architecture/adr-split-api-gateway-boundary" {
		t.Fatalf("expected architecture family inference, got %q", arch)
	}
}

func TestTopicKeyUpsertIsScopedByProjectAndScope(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	baseID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "architecture",
		Title:     "Auth model",
		Content:   "Initial architecture",
		Project:   "mnemo",
		Scope:     "project",
		TopicKey:  "architecture/auth-model",
	})
	if err != nil {
		t.Fatalf("add base observation: %v", err)
	}

	personalID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "architecture",
		Title:     "Auth model",
		Content:   "Personal take",
		Project:   "mnemo",
		Scope:     "personal",
		TopicKey:  "architecture/auth-model",
	})
	if err != nil {
		t.Fatalf("add personal scoped observation: %v", err)
	}

	otherProjectID, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "architecture",
		Title:     "Auth model",
		Content:   "Other project",
		Project:   "another-project",
		Scope:     "project",
		TopicKey:  "architecture/auth-model",
	})
	if err != nil {
		t.Fatalf("add other project observation: %v", err)
	}

	if baseID == personalID || baseID == otherProjectID || personalID == otherProjectID {
		t.Fatalf("expected topic upsert boundaries by project+scope, got ids base=%d personal=%d other=%d", baseID, personalID, otherProjectID)
	}
}

func TestPromptProjectNullScan(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Manually insert a prompt with NULL project to simulate legacy data or external changes
	_, err := s.db.Exec(
		"INSERT INTO user_prompts (session_id, content, project) VALUES (?, ?, NULL)",
		"s1", "prompt with null project",
	)
	if err != nil {
		t.Fatalf("manual insert: %v", err)
	}

	// 1. Test RecentPrompts
	prompts, err := s.RecentPrompts("", 10)
	if err != nil {
		t.Fatalf("RecentPrompts failed with null project: %v", err)
	}
	if len(prompts) != 1 || prompts[0].Project != "" {
		t.Errorf("expected empty string for null project, got %q", prompts[0].Project)
	}

	// 2. Test SearchPrompts
	searchResult, err := s.SearchPrompts("null", "", 10)
	if err != nil {
		t.Fatalf("SearchPrompts failed with null project: %v", err)
	}
	if len(searchResult) != 1 || searchResult[0].Project != "" {
		t.Errorf("expected empty string for null project in search, got %q", searchResult[0].Project)
	}

	// 3. Test Export
	data, err := s.Export()
	if err != nil {
		t.Fatalf("Export failed with null project: %v", err)
	}
	found := false
	for _, p := range data.Prompts {
		if p.Content == "prompt with null project" {
			found = true
			if p.Project != "" {
				t.Errorf("expected empty string for null project in export, got %q", p.Project)
			}
		}
	}
	if !found {
		t.Error("exported prompts missing the test prompt")
	}
}

// ─── Passive Capture Tests ───────────────────────────────────────────────────

func TestExtractLearningsNumberedList(t *testing.T) {
	text := `Some preamble text here.

## Key Learnings:

1. bcrypt cost=12 is the right balance for our server performance
2. JWT refresh tokens need atomic rotation to prevent race conditions
3. Always validate the audience claim in JWT tokens before trusting them

## Next Steps
- something else
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 3 {
		t.Fatalf("expected 3 learnings, got %d: %v", len(learnings), learnings)
	}
	if !strings.Contains(learnings[0], "bcrypt") {
		t.Fatalf("expected first learning about bcrypt, got %q", learnings[0])
	}
}

func TestExtractLearningsSpanishHeader(t *testing.T) {
	text := `## Aprendizajes Clave:

1. El costo de bcrypt=12 es el balance correcto para nuestro servidor
2. Los refresh tokens de JWT necesitan rotacion atomica
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 2 {
		t.Fatalf("expected 2 learnings, got %d: %v", len(learnings), learnings)
	}
}

func TestExtractLearningsBulletList(t *testing.T) {
	text := `### Learnings:

- bcrypt cost=12 is the right balance for our server performance
- JWT refresh tokens need atomic rotation to prevent race conditions
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 2 {
		t.Fatalf("expected 2 learnings, got %d: %v", len(learnings), learnings)
	}
}

func TestExtractLearningsIgnoresShortItems(t *testing.T) {
	text := `## Key Learnings:

1. too short
2. bcrypt cost=12 is the right balance for our server performance
3. also short
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 1 {
		t.Fatalf("expected 1 learning (short ones filtered), got %d: %v", len(learnings), learnings)
	}
}

func TestExtractLearningsNoSection(t *testing.T) {
	text := `This is just regular text without any learning section headers.
It has multiple lines but no ## Key Learnings or similar.
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 0 {
		t.Fatalf("expected 0 learnings, got %d: %v", len(learnings), learnings)
	}
}

func TestExtractLearningsSectionPresentButNoValidItems(t *testing.T) {
	text := `## Key Learnings:

1. short
2. tiny
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 0 {
		t.Fatalf("expected 0 learnings when section has no valid items, got %d: %v", len(learnings), learnings)
	}
}

func TestExtractLearningsUsesLastSection(t *testing.T) {
	text := `## Key Learnings:

1. This is from the first section and should be ignored

Some other text here.

## Key Learnings:

1. This is from the last section and should be captured as the real one
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 1 {
		t.Fatalf("expected 1 learning from last section, got %d: %v", len(learnings), learnings)
	}
	if !strings.Contains(learnings[0], "last section") {
		t.Fatalf("expected learning from last section, got %q", learnings[0])
	}
}

func TestExtractLearningsFallsBackWhenLastSectionHasNoValidItems(t *testing.T) {
	text := `## Key Learnings:

1. This is long enough and should be captured from the previous section

## Key Learnings:

1. short
2. tiny
`
	learnings := ExtractLearnings(text)
	if len(learnings) != 1 {
		t.Fatalf("expected fallback to previous valid section, got %d: %v", len(learnings), learnings)
	}
	if !strings.Contains(learnings[0], "previous section") {
		t.Fatalf("expected learning from previous section, got %q", learnings[0])
	}
}

func TestExtractLearningsCleansMarkdown(t *testing.T) {
	text := "## Key Learnings:\n\n1. **Use** `context.Context` in *all* handlers to support cancellation correctly\n"
	learnings := ExtractLearnings(text)
	if len(learnings) != 1 {
		t.Fatalf("expected 1 learning, got %d: %v", len(learnings), learnings)
	}
	if strings.Contains(learnings[0], "**") || strings.Contains(learnings[0], "`") || strings.Contains(learnings[0], "*") {
		t.Fatalf("expected markdown to be stripped, got %q", learnings[0])
	}
}

func TestPassiveCaptureStoresLearnings(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	text := `## Key Learnings:

1. bcrypt cost=12 is the right balance for our server performance
2. JWT refresh tokens need atomic rotation to prevent race conditions
`
	result, err := s.PassiveCapture(PassiveCaptureParams{
		SessionID: "s1",
		Content:   text,
		Project:   "mnemo",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("passive capture: %v", err)
	}
	if result.Extracted != 2 {
		t.Fatalf("expected 2 extracted, got %d", result.Extracted)
	}
	if result.Saved != 2 {
		t.Fatalf("expected 2 saved, got %d", result.Saved)
	}

	obs, err := s.AllObservations("mnemo", "", 10)
	if err != nil {
		t.Fatalf("all observations: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	for _, o := range obs {
		if o.Type != "passive" {
			t.Fatalf("expected type=passive, got %q", o.Type)
		}
	}
	if obs[0].ToolName == nil || *obs[0].ToolName != "test" {
		t.Fatalf("expected tool_name source to be stored as 'test', got %+v", obs[0].ToolName)
	}
}

func TestPassiveCaptureEmptyContent(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	result, err := s.PassiveCapture(PassiveCaptureParams{
		SessionID: "s1",
		Content:   "",
		Project:   "mnemo",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("passive capture: %v", err)
	}
	if result.Extracted != 0 || result.Saved != 0 {
		t.Fatalf("expected 0 extracted and 0 saved, got %d/%d", result.Extracted, result.Saved)
	}
}

func TestPassiveCaptureDedupesAgainstExistingObservations(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// First: agent saves actively via mem_save
	_, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "decision",
		Title:     "bcrypt cost",
		Content:   "bcrypt cost=12 is the right balance for our server performance",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add active observation: %v", err)
	}

	// Then: passive capture fires with overlapping content
	text := `## Key Learnings:

1. bcrypt cost=12 is the right balance for our server performance
2. JWT refresh tokens need atomic rotation to prevent race conditions
`
	result, err := s.PassiveCapture(PassiveCaptureParams{
		SessionID: "s1",
		Content:   text,
		Project:   "mnemo",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("passive capture: %v", err)
	}
	if result.Extracted != 2 {
		t.Fatalf("expected 2 extracted, got %d", result.Extracted)
	}
	if result.Saved != 1 {
		t.Fatalf("expected 1 saved (1 deduped), got %d", result.Saved)
	}
	if result.Duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", result.Duplicates)
	}
}

func TestPassiveCaptureReturnsErrorWhenSessionDoesNotExist(t *testing.T) {
	s := newTestStore(t)

	text := `## Key Learnings:

1. This learning is long enough to attempt insert and fail without session
`
	_, err := s.PassiveCapture(PassiveCaptureParams{
		SessionID: "missing-session",
		Content:   text,
		Project:   "mnemo",
		Source:    "test",
	})
	if err == nil {
		t.Fatalf("expected error when session does not exist")
	}
}

func TestStatsProjectsOrderedByMostRecentObservation(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session s1: %v", err)
	}
	if err := s.CreateSession("s2", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session s2: %v", err)
	}

	_, err := s.db.Exec(
		`INSERT INTO observations (session_id, type, title, content, project, scope, normalized_hash, revision_count, duplicate_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?),
		        (?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?)`,
		"s1", "note", "older", "older alpha", "alpha", "project", hashNormalized("older alpha"), "2026-02-01 10:00:00", "2026-02-01 10:00:00",
		"s2", "note", "newer", "newer beta", "beta", "project", hashNormalized("newer beta"), "2026-02-02 10:00:00", "2026-02-02 10:00:00",
	)
	if err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(stats.Projects) < 2 {
		t.Fatalf("expected at least 2 projects, got %d", len(stats.Projects))
	}

	if stats.Projects[0] != "beta" || stats.Projects[1] != "alpha" {
		t.Fatalf("expected recency order [beta alpha], got %v", stats.Projects[:2])
	}
}

func TestSessionsOrderedByMostRecentActivity(t *testing.T) {
	s := newTestStore(t)

	_, err := s.db.Exec(
		`INSERT INTO sessions (id, project, directory, started_at) VALUES
		 (?, ?, ?, ?),
		 (?, ?, ?, ?)`,
		"s-older", "mnemo", "/tmp/mnemo", "2026-02-01 09:00:00",
		"s-newer", "mnemo", "/tmp/mnemo", "2026-02-02 09:00:00",
	)
	if err != nil {
		t.Fatalf("insert sessions: %v", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO observations (session_id, type, title, content, project, scope, normalized_hash, revision_count, duplicate_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?)`,
		"s-older", "note", "latest", "session old got new activity", "mnemo", "project", hashNormalized("session old got new activity"), "2026-02-03 09:00:00", "2026-02-03 09:00:00",
	)
	if err != nil {
		t.Fatalf("insert latest observation: %v", err)
	}

	all, err := s.AllSessions("", 10)
	if err != nil {
		t.Fatalf("all sessions: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("expected at least 2 sessions, got %d", len(all))
	}
	if all[0].ID != "s-older" {
		t.Fatalf("expected s-older first in all sessions, got %s", all[0].ID)
	}

	recent, err := s.RecentSessions("", 10)
	if err != nil {
		t.Fatalf("recent sessions: %v", err)
	}
	if len(recent) < 2 {
		t.Fatalf("expected at least 2 recent sessions, got %d", len(recent))
	}
	if recent[0].ID != "s-older" {
		t.Fatalf("expected s-older first in recent sessions, got %s", recent[0].ID)
	}
}

func TestSessionObservationsAddPromptImportAndSyncChunks(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s1", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := s.AddObservation(AddObservationParams{
		SessionID: "s1",
		Type:      "decision",
		Title:     "Auth",
		Content:   "Use middleware chain",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add observation: %v", err)
	}

	longPrompt := strings.Repeat("x", s.cfg.MaxObservationLength+25)
	promptID, err := s.AddPrompt(AddPromptParams{SessionID: "s1", Content: longPrompt, Project: "mnemo"})
	if err != nil {
		t.Fatalf("add prompt: %v", err)
	}
	if promptID <= 0 {
		t.Fatalf("expected valid prompt id, got %d", promptID)
	}

	sessionObs, err := s.SessionObservations("s1", 0)
	if err != nil {
		t.Fatalf("session observations: %v", err)
	}
	if len(sessionObs) != 1 {
		t.Fatalf("expected 1 session observation, got %d", len(sessionObs))
	}

	exported, err := s.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	cfg := mustDefaultConfig(t)
	cfg.DataDir = t.TempDir()
	dst, err := New(cfg)
	if err != nil {
		t.Fatalf("new destination store: %v", err)
	}
	t.Cleanup(func() { _ = dst.Close() })

	imported, err := dst.Import(exported)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.SessionsImported < 1 || imported.ObservationsImported < 1 || imported.PromptsImported < 1 {
		t.Fatalf("expected non-zero import counts, got %+v", imported)
	}

	if err := dst.RecordSyncedChunk("chunk-1"); err != nil {
		t.Fatalf("record synced chunk: %v", err)
	}
	chunks, err := dst.GetSyncedChunks()
	if err != nil {
		t.Fatalf("get synced chunks: %v", err)
	}
	if !chunks["chunk-1"] {
		t.Fatalf("expected chunk-1 to be marked as synced")
	}
}

func TestStoreLocalSyncFoundationEnqueuesCoreMutations(t *testing.T) {
	s := newTestStore(t)

	// Enroll "mnemo" so mutations are visible via ListPendingSyncMutations.
	if err := s.EnrollProject("mnemo"); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if err := s.CreateSession("sync-session", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	obsID, err := s.AddObservation(AddObservationParams{
		SessionID: "sync-session",
		Type:      "decision",
		Title:     "Initial title",
		Content:   "Initial content",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add observation: %v", err)
	}

	updatedTitle := "Updated title"
	updatedContent := "Updated content"
	if _, err := s.UpdateObservation(obsID, UpdateObservationParams{
		Title:   &updatedTitle,
		Content: &updatedContent,
	}); err != nil {
		t.Fatalf("update observation: %v", err)
	}

	if err := s.DeleteObservation(obsID, false); err != nil {
		t.Fatalf("soft delete observation: %v", err)
	}

	promptID, err := s.AddPrompt(AddPromptParams{
		SessionID: "sync-session",
		Content:   "How do we keep this local-first?",
		Project:   "mnemo",
	})
	if err != nil {
		t.Fatalf("add prompt: %v", err)
	}

	if err := s.EndSession("sync-session", "done"); err != nil {
		t.Fatalf("end session: %v", err)
	}

	state, err := s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("get sync state: %v", err)
	}
	if state.TargetKey != DefaultSyncTargetKey {
		t.Fatalf("expected target %q, got %q", DefaultSyncTargetKey, state.TargetKey)
	}
	if state.Lifecycle != SyncLifecyclePending {
		t.Fatalf("expected pending lifecycle after local writes, got %q", state.Lifecycle)
	}
	if state.LastEnqueuedSeq != 6 {
		t.Fatalf("expected 6 enqueued mutations, got %d", state.LastEnqueuedSeq)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 10)
	if err != nil {
		t.Fatalf("list pending sync mutations: %v", err)
	}
	if len(mutations) != 6 {
		t.Fatalf("expected 6 pending mutations, got %d", len(mutations))
	}

	var observationSyncID string
	if err := s.db.QueryRow("SELECT sync_id FROM observations WHERE id = ?", obsID).Scan(&observationSyncID); err != nil {
		t.Fatalf("lookup observation sync id: %v", err)
	}
	if observationSyncID == "" {
		t.Fatalf("expected observation sync id to be persisted")
	}

	var promptSyncID string
	if err := s.db.QueryRow("SELECT sync_id FROM user_prompts WHERE id = ?", promptID).Scan(&promptSyncID); err != nil {
		t.Fatalf("lookup prompt sync id: %v", err)
	}
	if promptSyncID == "" {
		t.Fatalf("expected prompt sync id to be persisted")
	}

	if mutations[0].Entity != SyncEntitySession || mutations[0].EntityKey != "sync-session" || mutations[0].Op != SyncOpUpsert {
		t.Fatalf("unexpected session mutation: %+v", mutations[0])
	}
	if mutations[1].Entity != SyncEntityObservation || mutations[1].EntityKey != observationSyncID || mutations[1].Op != SyncOpUpsert {
		t.Fatalf("unexpected observation insert mutation: %+v", mutations[1])
	}
	if mutations[2].Entity != SyncEntityObservation || mutations[2].EntityKey != observationSyncID || mutations[2].Op != SyncOpUpsert {
		t.Fatalf("unexpected observation update mutation: %+v", mutations[2])
	}
	if mutations[3].Entity != SyncEntityObservation || mutations[3].EntityKey != observationSyncID || mutations[3].Op != SyncOpDelete {
		t.Fatalf("unexpected observation delete mutation: %+v", mutations[3])
	}
	if mutations[4].Entity != SyncEntityPrompt || mutations[4].EntityKey != promptSyncID || mutations[4].Op != SyncOpUpsert {
		t.Fatalf("unexpected prompt mutation: %+v", mutations[4])
	}
	if mutations[5].Entity != SyncEntitySession || mutations[5].EntityKey != "sync-session" || mutations[5].Op != SyncOpUpsert {
		t.Fatalf("unexpected end session mutation: %+v", mutations[5])
	}

	var deletedPayload map[string]any
	if err := json.Unmarshal([]byte(mutations[3].Payload), &deletedPayload); err != nil {
		t.Fatalf("decode delete payload: %v", err)
	}
	if deletedPayload["sync_id"] != observationSyncID {
		t.Fatalf("expected delete payload sync id %q, got %#v", observationSyncID, deletedPayload["sync_id"])
	}
	if deletedPayload["deleted"] != true {
		t.Fatalf("expected delete payload to mark deleted=true, got %#v", deletedPayload["deleted"])
	}

	if err := s.AckSyncMutations(DefaultSyncTargetKey, mutations[3].Seq); err != nil {
		t.Fatalf("ack sync mutations: %v", err)
	}
	remaining, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 10)
	if err != nil {
		t.Fatalf("list remaining sync mutations: %v", err)
	}
	if len(remaining) != 2 || remaining[0].Entity != SyncEntityPrompt || remaining[1].Entity != SyncEntitySession {
		t.Fatalf("expected prompt and end-session mutations to remain pending, got %+v", remaining)
	}
}

func TestStoreLocalSyncFoundationStateHelpers(t *testing.T) {
	s := newTestStore(t)

	state, err := s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("get initial sync state: %v", err)
	}
	if state.Lifecycle != SyncLifecycleIdle {
		t.Fatalf("expected idle lifecycle, got %q", state.Lifecycle)
	}

	acquired, err := s.AcquireSyncLease(DefaultSyncTargetKey, "worker-a", 2*time.Minute, time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}
	if !acquired {
		t.Fatalf("expected first lease acquisition to succeed")
	}

	acquired, err = s.AcquireSyncLease(DefaultSyncTargetKey, "worker-b", 2*time.Minute, time.Date(2026, 3, 7, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("acquire conflicting lease: %v", err)
	}
	if acquired {
		t.Fatalf("expected conflicting lease acquisition to fail")
	}

	if err := s.ReleaseSyncLease(DefaultSyncTargetKey, "worker-a"); err != nil {
		t.Fatalf("release lease: %v", err)
	}

	acquired, err = s.AcquireSyncLease(DefaultSyncTargetKey, "worker-b", 2*time.Minute, time.Date(2026, 3, 7, 12, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("acquire released lease: %v", err)
	}
	if !acquired {
		t.Fatalf("expected lease acquisition after release to succeed")
	}

	if err := s.MarkSyncFailure(DefaultSyncTargetKey, "timeout talking to cloud", time.Date(2026, 3, 7, 12, 10, 0, 0, time.UTC)); err != nil {
		t.Fatalf("mark sync failure: %v", err)
	}

	state, err = s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("get degraded sync state: %v", err)
	}
	if state.Lifecycle != SyncLifecycleDegraded {
		t.Fatalf("expected degraded lifecycle, got %q", state.Lifecycle)
	}
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("expected failure count 1, got %d", state.ConsecutiveFailures)
	}
	if state.LastError == nil || *state.LastError != "timeout talking to cloud" {
		t.Fatalf("expected last error to be stored, got %+v", state.LastError)
	}
	if state.BackoffUntil == nil || *state.BackoffUntil != "2026-03-07T12:10:00Z" {
		t.Fatalf("expected backoff timestamp to be stored, got %+v", state.BackoffUntil)
	}

	if err := s.MarkSyncHealthy(DefaultSyncTargetKey); err != nil {
		t.Fatalf("mark sync healthy: %v", err)
	}

	state, err = s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("get healthy sync state: %v", err)
	}
	if state.Lifecycle != SyncLifecycleHealthy {
		t.Fatalf("expected healthy lifecycle, got %q", state.Lifecycle)
	}
	if state.ConsecutiveFailures != 0 || state.LastError != nil || state.BackoffUntil != nil {
		t.Fatalf("expected healthy state to clear failure metadata, got %+v", state)
	}
}

func TestApplyRemoteMutationIdempotent(t *testing.T) {
	s := newTestStore(t)

	create := SyncMutation{
		Seq:       41,
		TargetKey: DefaultSyncTargetKey,
		Entity:    SyncEntitySession,
		EntityKey: "remote-session",
		Op:        SyncOpUpsert,
		Payload:   `{"id":"remote-session","project":"mnemo","directory":"/remote"}`,
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, create); err != nil {
		t.Fatalf("apply session mutation: %v", err)
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, create); err != nil {
		t.Fatalf("reapply session mutation: %v", err)
	}

	obsMutation := SyncMutation{
		Seq:       42,
		TargetKey: DefaultSyncTargetKey,
		Entity:    SyncEntityObservation,
		EntityKey: "obs-remote-1",
		Op:        SyncOpUpsert,
		Payload:   `{"sync_id":"obs-remote-1","session_id":"remote-session","type":"decision","title":"Remote","content":"Pulled from cloud","project":"mnemo","scope":"project"}`,
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, obsMutation); err != nil {
		t.Fatalf("apply observation mutation: %v", err)
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, obsMutation); err != nil {
		t.Fatalf("reapply observation mutation: %v", err)
	}

	var rowCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM observations WHERE sync_id = ?", "obs-remote-1").Scan(&rowCount); err != nil {
		t.Fatalf("count remote observation rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected one remote observation row after idempotent upsert, got %d", rowCount)
	}

	deleteMutation := SyncMutation{
		Seq:       43,
		TargetKey: DefaultSyncTargetKey,
		Entity:    SyncEntityObservation,
		EntityKey: "obs-remote-1",
		Op:        SyncOpDelete,
		Payload:   `{"sync_id":"obs-remote-1","deleted":true}`,
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, deleteMutation); err != nil {
		t.Fatalf("apply delete mutation: %v", err)
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, deleteMutation); err != nil {
		t.Fatalf("reapply delete mutation: %v", err)
	}

	if _, err := s.GetObservationBySyncID("obs-remote-1"); err == nil {
		t.Fatalf("expected pulled delete to hide observation")
	}

	pending, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 10)
	if err != nil {
		t.Fatalf("list pending after pulled apply: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected pulled apply helpers to avoid local re-enqueue, got %+v", pending)
	}

	state, err := s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("get sync state after pulled apply: %v", err)
	}
	if state.LastPulledSeq != 43 {
		t.Fatalf("expected last pulled seq 43, got %d", state.LastPulledSeq)
	}
}

func TestApplyPulledMutationAcceptsStringifiedSessionPayload(t *testing.T) {
	s := newTestStore(t)

	mutation := SyncMutation{
		Seq:       1,
		TargetKey: DefaultSyncTargetKey,
		Entity:    SyncEntitySession,
		EntityKey: "remote-session",
		Op:        SyncOpUpsert,
		Payload:   `"{\"id\":\"remote-session\",\"project\":\"mnemo\",\"directory\":\"/remote\"}"`,
	}
	if err := s.ApplyPulledMutation(DefaultSyncTargetKey, mutation); err != nil {
		t.Fatalf("apply stringified session mutation: %v", err)
	}

	session, err := s.GetSession("remote-session")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Project != "mnemo" || session.Directory != "/remote" {
		t.Fatalf("unexpected session after pulled apply: %+v", session)
	}
}

func TestUtilityHelpersCoverage(t *testing.T) {
	if got := derefString(nil); got != "" {
		t.Fatalf("expected empty string for nil pointer, got %q", got)
	}
	v := "value"
	if got := derefString(&v); got != "value" {
		t.Fatalf("expected dereferenced value, got %q", got)
	}

	if got := maxInt(10, 5); got != 10 {
		t.Fatalf("expected maxInt(10,5)=10, got %d", got)
	}
	if got := maxInt(3, 7); got != 7 {
		t.Fatalf("expected maxInt(3,7)=7, got %d", got)
	}

	if got := dedupeWindowExpression(0); got != "-15 minutes" {
		t.Fatalf("expected default dedupe window, got %q", got)
	}
	if got := dedupeWindowExpression(20 * time.Second); got != "-1 minutes" {
		t.Fatalf("expected minimum 1 minute window, got %q", got)
	}

	cases := map[string]string{
		"write":   "file_change",
		"patch":   "file_change",
		"bash":    "command",
		"read":    "file_read",
		"glob":    "search",
		"unknown": "tool_use",
	}
	for in, want := range cases {
		if got := ClassifyTool(in); got != want {
			t.Fatalf("ClassifyTool(%q): expected %q, got %q", in, want, got)
		}
	}
}

func TestEndSessionAndTimelineDefaults(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-end", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	firstID, err := s.AddObservation(AddObservationParams{
		SessionID: "s-end",
		Type:      "note",
		Title:     "first",
		Content:   "first note",
		Project:   "mnemo",
	})
	if err != nil {
		t.Fatalf("add first observation: %v", err)
	}
	_, err = s.AddObservation(AddObservationParams{
		SessionID: "s-end",
		Type:      "note",
		Title:     "second",
		Content:   "second note",
		Project:   "mnemo",
	})
	if err != nil {
		t.Fatalf("add second observation: %v", err)
	}

	if err := s.EndSession("s-end", "finished session"); err != nil {
		t.Fatalf("end session: %v", err)
	}

	sess, err := s.GetSession("s-end")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.EndedAt == nil {
		t.Fatalf("expected ended_at to be set")
	}
	if sess.Summary == nil || *sess.Summary != "finished session" {
		t.Fatalf("expected summary to be stored, got %+v", sess.Summary)
	}

	timeline, err := s.Timeline(firstID, 0, -1)
	if err != nil {
		t.Fatalf("timeline with default before/after: %v", err)
	}
	if timeline.SessionInfo == nil {
		t.Fatalf("expected session info in timeline")
	}
	if timeline.TotalInRange != 2 {
		t.Fatalf("expected total_in_range=2, got %d", timeline.TotalInRange)
	}
}

func TestInferTopicFamilyCoverage(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		title   string
		content string
		want    string
	}{
		{name: "type architecture", typ: "architecture", want: "architecture"},
		{name: "type bugfix", typ: "bugfix", want: "bug"},
		{name: "type decision", typ: "decision", want: "decision"},
		{name: "type pattern", typ: "pattern", want: "pattern"},
		{name: "type config", typ: "config", want: "config"},
		{name: "type discovery", typ: "discovery", want: "discovery"},
		{name: "type learning", typ: "learning", want: "learning"},
		{name: "type session summary", typ: "session_summary", want: "session"},
		{name: "text bug", title: "", content: "this caused a crash regression", want: "bug"},
		{name: "text architecture", title: "", content: "new boundary design", want: "architecture"},
		{name: "text decision", title: "", content: "we chose this tradeoff", want: "decision"},
		{name: "text pattern", title: "", content: "naming convention for handlers", want: "pattern"},
		{name: "text config", title: "", content: "docker env setup", want: "config"},
		{name: "text discovery", title: "", content: "root cause found", want: "discovery"},
		{name: "text learning", title: "", content: "key learning from this issue", want: "learning"},
		{name: "fallback type", typ: "Custom Type", want: "custom-type"},
		{name: "default topic", typ: "manual", title: "", content: "", want: "topic"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inferTopicFamily(tc.typ, tc.title, tc.content)
			if got != tc.want {
				t.Fatalf("inferTopicFamily(%q,%q,%q): expected %q, got %q", tc.typ, tc.title, tc.content, tc.want, got)
			}
		})
	}
}

func TestStoreAdditionalQueryAndMutationBranches(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-q", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	longContent := strings.Repeat("x", s.cfg.MaxObservationLength+100)
	obsID, err := s.AddObservation(AddObservationParams{
		SessionID: "s-q",
		Type:      "note",
		Title:     "Private <private>secret</private> title",
		Content:   longContent + " <private>token</private>",
		Project:   "mnemo",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add observation: %v", err)
	}
	obs, err := s.GetObservation(obsID)
	if err != nil {
		t.Fatalf("get observation: %v", err)
	}
	if !strings.Contains(obs.Title, "[REDACTED]") {
		t.Fatalf("expected private tags redacted in title, got %q", obs.Title)
	}
	if !strings.Contains(obs.Content, "... [truncated]") {
		t.Fatalf("expected truncated content marker, got %q", obs.Content)
	}

	newProject := ""
	newTopic := ""
	updated, err := s.UpdateObservation(obsID, UpdateObservationParams{Project: &newProject, TopicKey: &newTopic})
	if err != nil {
		t.Fatalf("update observation: %v", err)
	}
	if updated.Project != nil {
		t.Fatalf("expected nil project after empty update")
	}
	if updated.TopicKey != nil {
		t.Fatalf("expected nil topic key after empty update")
	}

	if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-q", Content: "alpha prompt", Project: "alpha"}); err != nil {
		t.Fatalf("add alpha prompt: %v", err)
	}
	if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-q", Content: "beta prompt", Project: "beta"}); err != nil {
		t.Fatalf("add beta prompt: %v", err)
	}

	recentPrompts, err := s.RecentPrompts("beta", 1)
	if err != nil {
		t.Fatalf("recent prompts with project filter: %v", err)
	}
	if len(recentPrompts) != 1 || recentPrompts[0].Project != "beta" {
		t.Fatalf("expected one beta prompt, got %+v", recentPrompts)
	}

	searchPrompts, err := s.SearchPrompts("prompt", "alpha", 0)
	if err != nil {
		t.Fatalf("search prompts with project filter/default limit: %v", err)
	}
	if len(searchPrompts) != 1 || searchPrompts[0].Project != "alpha" {
		t.Fatalf("expected one alpha prompt search result, got %+v", searchPrompts)
	}

	searchResults, err := s.Search("title", SearchOptions{Scope: "project", Limit: 9999})
	if err != nil {
		t.Fatalf("search with clamped limit: %v", err)
	}
	if len(searchResults) == 0 {
		t.Fatalf("expected search results")
	}

	ctx, err := s.FormatContext("", "project")
	if err != nil {
		t.Fatalf("format context: %v", err)
	}
	if !strings.Contains(ctx, "Recent User Prompts") {
		t.Fatalf("expected prompts section in context output")
	}
}

func TestStoreErrorBranchesWithClosedDatabase(t *testing.T) {
	s := newTestStore(t)

	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if _, err := s.GetSession("missing"); err == nil {
		t.Fatalf("expected GetSession error when db is closed")
	}
	if _, err := s.AllSessions("", 1); err == nil {
		t.Fatalf("expected AllSessions error when db is closed")
	}
	if _, err := s.RecentSessions("", 1); err == nil {
		t.Fatalf("expected RecentSessions error when db is closed")
	}
	if _, err := s.SearchPrompts("x", "", 1); err == nil {
		t.Fatalf("expected SearchPrompts error when db is closed")
	}
	if _, err := s.Search("x", SearchOptions{}); err == nil {
		t.Fatalf("expected Search error when db is closed")
	}
	if _, err := s.Export(); err == nil {
		t.Fatalf("expected Export error when db is closed")
	}
	if _, err := s.Timeline(1, 1, 1); err == nil {
		t.Fatalf("expected Timeline error when db is closed")
	}
}

func TestEndSessionEdgeCases(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-edge", "mnemo", "/tmp/mnemo"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := s.EndSession("missing", "ignored"); err != nil {
		t.Fatalf("end missing session should be no-op: %v", err)
	}

	if err := s.EndSession("s-edge", ""); err != nil {
		t.Fatalf("end session with empty summary: %v", err)
	}

	sess, err := s.GetSession("s-edge")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.EndedAt == nil {
		t.Fatalf("expected ended_at to be set")
	}
	if sess.Summary != nil {
		t.Fatalf("expected empty summary to persist as NULL, got %q", *sess.Summary)
	}
}

func TestTimelineHandlesMissingSessionRecord(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable fk: %v", err)
	}
	defer func() {
		_, _ = s.db.Exec("PRAGMA foreign_keys = ON")
	}()

	res, err := s.db.Exec(
		`INSERT INTO observations (session_id, type, title, content, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, 1, datetime('now'), datetime('now'))`,
		"manual-save", "manual", "orphan", "orphan content", "mnemo", "project", hashNormalized("orphan content"),
	)
	if err != nil {
		t.Fatalf("insert orphan observation: %v", err)
	}
	obsID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	timeline, err := s.Timeline(obsID, 1, 1)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if timeline.SessionInfo != nil {
		t.Fatalf("expected nil session info for missing session, got %+v", timeline.SessionInfo)
	}
	if timeline.TotalInRange != 1 {
		t.Fatalf("expected total in range=1, got %d", timeline.TotalInRange)
	}
}

func TestQueryObservationsScanError(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.queryObservations("SELECT 1"); err == nil {
		t.Fatalf("expected scan error for mismatched projection")
	}
}

func TestMigrationAndHelperEdgeBranches(t *testing.T) {
	t.Run("migrate is idempotent with existing triggers", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.migrate(); err != nil {
			t.Fatalf("second migrate should succeed: %v", err)
		}
	})

	t.Run("legacy migrate skips table without id column", func(t *testing.T) {
		s := newTestStore(t)

		if _, err := s.db.Exec(`
			DROP TRIGGER IF EXISTS obs_fts_insert;
			DROP TRIGGER IF EXISTS obs_fts_update;
			DROP TRIGGER IF EXISTS obs_fts_delete;
			DROP TABLE IF EXISTS observations_fts;
			DROP TABLE observations;
			CREATE TABLE observations (
				session_id TEXT,
				type TEXT,
				title TEXT,
				content TEXT
			);
		`); err != nil {
			t.Fatalf("recreate observations without id: %v", err)
		}

		if err := s.migrateLegacyObservationsTable(); err != nil {
			t.Fatalf("legacy migrate should skip tables without id: %v", err)
		}
	})

	t.Run("topic helpers normalize edge cases", func(t *testing.T) {
		if got := SuggestTopicKey("decision", "decision", ""); got != "decision/general" {
			t.Fatalf("expected decision/general, got %q", got)
		}
		if got := SuggestTopicKey("bugfix", "bug-auth-panic", ""); got != "bug/auth-panic" {
			t.Fatalf("expected bug/auth-panic, got %q", got)
		}
		if got := SuggestTopicKey("manual", "!!!", "..."); got != "topic/general" {
			t.Fatalf("expected topic/general fallback, got %q", got)
		}

		longSegment := normalizeTopicSegment(strings.Repeat("abc", 50))
		if len(longSegment) != 100 {
			t.Fatalf("expected topic segment truncation to 100, got %d", len(longSegment))
		}

		longKey := normalizeTopicKey(strings.Repeat("k", 200))
		if len(longKey) != 120 {
			t.Fatalf("expected topic key truncation to 120, got %d", len(longKey))
		}
	})

	t.Run("format context empty returns empty string", func(t *testing.T) {
		s := newTestStore(t)
		ctx, err := s.FormatContext("", "")
		if err != nil {
			t.Fatalf("format context: %v", err)
		}
		if ctx != "" {
			t.Fatalf("expected empty context when no data, got %q", ctx)
		}
	})
}

func TestExportImportEdgeBranches(t *testing.T) {
	t.Run("export fails when observations query fails", func(t *testing.T) {
		s := newTestStore(t)

		if _, err := s.db.Exec(`
			DROP TRIGGER IF EXISTS obs_fts_insert;
			DROP TRIGGER IF EXISTS obs_fts_update;
			DROP TRIGGER IF EXISTS obs_fts_delete;
			DROP TABLE IF EXISTS observations_fts;
			DROP TABLE observations;
		`); err != nil {
			t.Fatalf("drop observations: %v", err)
		}

		_, err := s.Export()
		if err == nil || !strings.Contains(err.Error(), "export observations") {
			t.Fatalf("expected observations export error, got %v", err)
		}
	})

	t.Run("export fails when prompts query fails", func(t *testing.T) {
		s := newTestStore(t)

		if _, err := s.db.Exec(`
			DROP TRIGGER IF EXISTS prompt_fts_insert;
			DROP TRIGGER IF EXISTS prompt_fts_update;
			DROP TRIGGER IF EXISTS prompt_fts_delete;
			DROP TABLE IF EXISTS prompts_fts;
			DROP TABLE user_prompts;
		`); err != nil {
			t.Fatalf("drop prompts: %v", err)
		}

		_, err := s.Export()
		if err == nil || !strings.Contains(err.Error(), "export prompts") {
			t.Fatalf("expected prompts export error, got %v", err)
		}
	})

	t.Run("import begin tx fails on closed db", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}

		_, err := s.Import(&ExportData{})
		if err == nil || !strings.Contains(err.Error(), "begin tx") {
			t.Fatalf("expected begin tx import error, got %v", err)
		}
	})

	t.Run("import fails on observation fk error", func(t *testing.T) {
		s := newTestStore(t)
		_, err := s.Import(&ExportData{
			Observations: []Observation{{
				ID:        1,
				SessionID: "missing-session",
				Type:      "bugfix",
				Title:     "x",
				Content:   "y",
				Scope:     "project",
				CreatedAt: Now(),
				UpdatedAt: Now(),
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "import observation") {
			t.Fatalf("expected observation import error, got %v", err)
		}
	})

	t.Run("import fails on prompt fk error", func(t *testing.T) {
		s := newTestStore(t)
		_, err := s.Import(&ExportData{
			Prompts: []Prompt{{
				ID:        1,
				SessionID: "missing-session",
				Content:   "prompt",
				Project:   "mnemo",
				CreatedAt: Now(),
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "import prompt") {
			t.Fatalf("expected prompt import error, got %v", err)
		}
	})
}

func TestNewErrorBranches(t *testing.T) {
	t.Run("fails when data dir is a file", func(t *testing.T) {
		base := t.TempDir()
		badPath := filepath.Join(base, "not-a-dir")
		if err := os.WriteFile(badPath, []byte("x"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		cfg := mustDefaultConfig(t)
		cfg.DataDir = badPath

		_, err := New(cfg)
		if err == nil || !strings.Contains(err.Error(), "create data dir") {
			t.Fatalf("expected create data dir error, got %v", err)
		}
	})

	t.Run("fails when db path is a directory", func(t *testing.T) {
		dataDir := t.TempDir()
		dbAsDir := filepath.Join(dataDir, "memory.db")
		if err := os.Mkdir(dbAsDir, 0755); err != nil {
			t.Fatalf("mkdir db path: %v", err)
		}

		cfg := mustDefaultConfig(t)
		cfg.DataDir = dataDir

		_, err := New(cfg)
		if err == nil {
			t.Fatalf("expected New to fail when db path is a directory")
		}
	})

	t.Run("fails when migration encounters conflicting object", func(t *testing.T) {
		dataDir := t.TempDir()
		dbPath := filepath.Join(dataDir, "memory.db")

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		_, err = db.Exec(`
			CREATE TABLE sessions (
				id TEXT PRIMARY KEY,
				project TEXT NOT NULL,
				directory TEXT NOT NULL,
				started_at TEXT NOT NULL,
				ended_at TEXT,
				summary TEXT
			);
			CREATE TABLE user_prompts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				session_id TEXT NOT NULL,
				content TEXT NOT NULL,
				created_at TEXT NOT NULL
			);
		`)
		if err != nil {
			_ = db.Close()
			t.Fatalf("create conflicting view: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}

		cfg := mustDefaultConfig(t)
		cfg.DataDir = dataDir

		_, err = New(cfg)
		if err == nil || !strings.Contains(err.Error(), "migration") {
			t.Fatalf("expected migration error, got %v", err)
		}
	})
}

func TestMigrationInternalErrorAndNoopBranches(t *testing.T) {
	t.Run("addColumnIfNotExists adds then noops", func(t *testing.T) {
		s := newTestStore(t)
		if _, err := s.db.Exec(`CREATE TABLE extra_table (id INTEGER)`); err != nil {
			t.Fatalf("create extra table: %v", err)
		}

		if err := s.addColumnIfNotExists("extra_table", "name", "TEXT"); err != nil {
			t.Fatalf("add column: %v", err)
		}
		if err := s.addColumnIfNotExists("extra_table", "name", "TEXT"); err != nil {
			t.Fatalf("add existing column should noop: %v", err)
		}

		if err := s.addColumnIfNotExists("missing_table", "x", "TEXT"); err == nil {
			t.Fatalf("expected missing table error")
		}
	})

	t.Run("legacy migrate noops when id is primary key", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.migrateLegacyObservationsTable(); err != nil {
			t.Fatalf("expected noop for modern schema: %v", err)
		}
	})

	t.Run("legacy migrate fails if temp table already exists", func(t *testing.T) {
		s := newTestStore(t)
		if _, err := s.db.Exec(`
			DROP TRIGGER IF EXISTS obs_fts_insert;
			DROP TRIGGER IF EXISTS obs_fts_update;
			DROP TRIGGER IF EXISTS obs_fts_delete;
			DROP TABLE IF EXISTS observations_fts;
			DROP TABLE observations;
			CREATE TABLE observations (
				id INT,
				session_id TEXT,
				type TEXT,
				title TEXT,
				content TEXT,
				created_at TEXT
			);
			CREATE TABLE observations_migrated (id INTEGER PRIMARY KEY);
		`); err != nil {
			t.Fatalf("prepare legacy schema: %v", err)
		}

		err := s.migrateLegacyObservationsTable()
		if err == nil || !strings.Contains(err.Error(), "create table") {
			t.Fatalf("expected create table error, got %v", err)
		}
	})

	t.Run("migrate returns deterministic exec hook errors", func(t *testing.T) {
		s := newTestStore(t)

		origExec := s.hooks.exec
		s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
			if strings.Contains(query, "UPDATE observations SET scope = 'project'") {
				return nil, errors.New("forced migrate update failure")
			}
			return origExec(db, query, args...)
		}

		err := s.migrate()
		if err == nil || !strings.Contains(err.Error(), "forced migrate update failure") {
			t.Fatalf("expected forced migrate failure, got %v", err)
		}
	})

	t.Run("migrate fails when creating missing triggers", func(t *testing.T) {
		s := newTestStore(t)

		if _, err := s.db.Exec(`
			DROP TRIGGER IF EXISTS obs_fts_insert;
			DROP TRIGGER IF EXISTS obs_fts_update;
			DROP TRIGGER IF EXISTS obs_fts_delete;
		`); err != nil {
			t.Fatalf("drop obs triggers: %v", err)
		}

		origExec := s.hooks.exec
		s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
			if strings.Contains(query, "CREATE TRIGGER obs_fts_insert") {
				return nil, errors.New("forced obs trigger failure")
			}
			return origExec(db, query, args...)
		}

		err := s.migrate()
		if err == nil || !strings.Contains(err.Error(), "forced obs trigger failure") {
			t.Fatalf("expected forced trigger failure, got %v", err)
		}
	})

	t.Run("legacy migrate surfaces begin and commit hook failures", func(t *testing.T) {
		prepareLegacyStore := func(t *testing.T) *Store {
			t.Helper()
			s := newTestStore(t)
			if _, err := s.db.Exec(`
				DROP TRIGGER IF EXISTS obs_fts_insert;
				DROP TRIGGER IF EXISTS obs_fts_update;
				DROP TRIGGER IF EXISTS obs_fts_delete;
				DROP TABLE IF EXISTS observations_fts;
				DROP TABLE observations;
				INSERT OR IGNORE INTO sessions (id, project, directory) VALUES ('s1', 'mnemo', '/tmp/mnemo');
				CREATE TABLE observations (
					id INT,
					session_id TEXT,
					type TEXT,
					title TEXT,
					content TEXT,
					tool_name TEXT,
					project TEXT,
					scope TEXT,
					topic_key TEXT,
					normalized_hash TEXT,
					revision_count INTEGER,
					duplicate_count INTEGER,
					last_seen_at TEXT,
					created_at TEXT,
					updated_at TEXT,
					deleted_at TEXT
				);
				INSERT INTO observations (id, session_id, type, title, content, project, created_at, updated_at)
				VALUES (1, 's1', 'bugfix', 'legacy', 'legacy row', 'mnemo', datetime('now'), datetime('now'));
			`); err != nil {
				t.Fatalf("prepare legacy table: %v", err)
			}
			return s
		}

		t.Run("begin tx", func(t *testing.T) {
			s := prepareLegacyStore(t)
			s.hooks.beginTx = func(_ *sql.DB) (*sql.Tx, error) {
				return nil, errors.New("forced begin failure")
			}

			err := s.migrateLegacyObservationsTable()
			if err == nil || !strings.Contains(err.Error(), "forced begin failure") {
				t.Fatalf("expected begin failure, got %v", err)
			}
		})

		t.Run("commit", func(t *testing.T) {
			s := prepareLegacyStore(t)
			s.hooks.commit = func(_ *sql.Tx) error {
				return errors.New("forced legacy commit failure")
			}

			err := s.migrateLegacyObservationsTable()
			if err == nil || !strings.Contains(err.Error(), "forced legacy commit failure") {
				t.Fatalf("expected commit failure, got %v", err)
			}
		})
	})
}

func TestImportExportSeamErrors(t *testing.T) {
	t.Run("export query hooks", func(t *testing.T) {
		s := newTestStore(t)

		origQueryIt := s.hooks.queryIt
		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "FROM sessions") {
				return nil, errors.New("forced sessions export query error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Export(); err == nil || !strings.Contains(err.Error(), "export sessions") {
			t.Fatalf("expected sessions export error, got %v", err)
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "FROM observations") {
				return nil, errors.New("forced observations export query error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Export(); err == nil || !strings.Contains(err.Error(), "export observations") {
			t.Fatalf("expected observations export error, got %v", err)
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "FROM user_prompts") {
				return nil, errors.New("forced prompts export query error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Export(); err == nil || !strings.Contains(err.Error(), "export prompts") {
			t.Fatalf("expected prompts export error, got %v", err)
		}
	})

	t.Run("import tx and exec hooks", func(t *testing.T) {
		s := newTestStore(t)

		s.hooks.beginTx = func(_ *sql.DB) (*sql.Tx, error) {
			return nil, errors.New("forced import begin failure")
		}
		if _, err := s.Import(&ExportData{}); err == nil || !strings.Contains(err.Error(), "begin tx") {
			t.Fatalf("expected begin tx error, got %v", err)
		}

		s.hooks = defaultStoreHooks()
		origExec := s.hooks.exec
		s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
			if strings.Contains(query, "INSERT OR IGNORE INTO sessions") {
				return nil, errors.New("forced import session insert failure")
			}
			return origExec(db, query, args...)
		}
		if _, err := s.Import(&ExportData{Sessions: []Session{{ID: "s-x", Project: "p", Directory: "/tmp", StartedAt: Now()}}}); err == nil || !strings.Contains(err.Error(), "import session") {
			t.Fatalf("expected session import error, got %v", err)
		}

		s.hooks = defaultStoreHooks()
		s.hooks.commit = func(_ *sql.Tx) error {
			return errors.New("forced import commit failure")
		}
		if _, err := s.Import(&ExportData{}); err == nil || !strings.Contains(err.Error(), "import: commit") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})
}

func TestHookFallbacksAndAdditionalBranches(t *testing.T) {
	t.Run("hook fallbacks call default DB methods", func(t *testing.T) {
		s := newTestStore(t)
		s.hooks = storeHooks{}

		if _, err := s.execHook(s.db, "SELECT 1"); err != nil {
			t.Fatalf("exec hook fallback: %v", err)
		}
		rows, err := s.queryHook(s.db, "SELECT 1")
		if err != nil {
			t.Fatalf("query hook fallback: %v", err)
		}
		_ = rows.Close()

		iter, err := s.queryItHook(s.db, "SELECT 1")
		if err != nil {
			t.Fatalf("query iterator fallback: %v", err)
		}
		_ = iter.Close()

		tx, err := s.beginTxHook()
		if err != nil {
			t.Fatalf("begin tx hook fallback: %v", err)
		}
		if err := s.commitHook(tx); err != nil {
			t.Fatalf("commit hook fallback: %v", err)
		}

		s2 := newTestStore(t)
		rows2, err := s2.queryHook(s2.db, "SELECT 1")
		if err != nil {
			t.Fatalf("query hook default closure: %v", err)
		}
		_ = rows2.Close()

		s.hooks.query = func(db queryer, query string, args ...any) (*sql.Rows, error) {
			return nil, errors.New("forced query hook error")
		}
		s.hooks.queryIt = nil
		if _, err := s.queryItHook(s.db, "SELECT 1"); err == nil {
			t.Fatalf("expected queryItHook error through queryHook fallback")
		}
	})

	t.Run("sessions and observations filters with default limits", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.CreateSession("s-p", "proj-a", "/tmp/proj-a"); err != nil {
			t.Fatalf("create session proj-a: %v", err)
		}
		if err := s.CreateSession("s-q", "proj-b", "/tmp/proj-b"); err != nil {
			t.Fatalf("create session proj-b: %v", err)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-p", Type: "note", Title: "a", Content: "a", Project: "proj-a", Scope: "project"}); err != nil {
			t.Fatalf("add observation proj-a: %v", err)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-q", Type: "note", Title: "b", Content: "b", Project: "proj-b", Scope: "project"}); err != nil {
			t.Fatalf("add observation proj-b: %v", err)
		}

		recent, err := s.RecentSessions("proj-a", 0)
		if err != nil {
			t.Fatalf("recent sessions filtered: %v", err)
		}
		if len(recent) != 1 || recent[0].Project != "proj-a" {
			t.Fatalf("expected one proj-a recent session, got %+v", recent)
		}

		all, err := s.AllSessions("proj-b", -1)
		if err != nil {
			t.Fatalf("all sessions filtered: %v", err)
		}
		if len(all) != 1 || all[0].Project != "proj-b" {
			t.Fatalf("expected one proj-b session, got %+v", all)
		}

		obs, err := s.AllObservations("proj-a", "project", 0)
		if err != nil {
			t.Fatalf("all observations defaults: %v", err)
		}
		if len(obs) != 1 || obs[0].SessionID != "s-p" {
			t.Fatalf("expected one proj-a observation, got %+v", obs)
		}

		sessionObs, err := s.SessionObservations("s-p", 0)
		if err != nil {
			t.Fatalf("session observations default limit: %v", err)
		}
		if len(sessionObs) != 1 {
			t.Fatalf("expected one session observation, got %d", len(sessionObs))
		}

		recentObs, err := s.RecentObservations("proj-a", "project", 0)
		if err != nil {
			t.Fatalf("recent observations default limit: %v", err)
		}
		if len(recentObs) != 1 {
			t.Fatalf("expected one recent observation, got %d", len(recentObs))
		}

		recentPrompts, err := s.RecentPrompts("", 0)
		if err != nil {
			t.Fatalf("recent prompts default limit: %v", err)
		}
		if len(recentPrompts) != 0 {
			t.Fatalf("expected zero prompts, got %d", len(recentPrompts))
		}
	})

	t.Run("timeline includes before and after in chronological order", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.CreateSession("s-tl", "mnemo", "/tmp/mnemo"); err != nil {
			t.Fatalf("create session: %v", err)
		}

		firstID, err := s.AddObservation(AddObservationParams{SessionID: "s-tl", Type: "note", Title: "1", Content: "one", Project: "mnemo"})
		if err != nil {
			t.Fatalf("add first observation: %v", err)
		}
		middleID, err := s.AddObservation(AddObservationParams{SessionID: "s-tl", Type: "note", Title: "2", Content: "two", Project: "mnemo"})
		if err != nil {
			t.Fatalf("add middle observation: %v", err)
		}
		lastID, err := s.AddObservation(AddObservationParams{SessionID: "s-tl", Type: "note", Title: "3", Content: "three", Project: "mnemo"})
		if err != nil {
			t.Fatalf("add last observation: %v", err)
		}

		tl, err := s.Timeline(middleID, 5, 5)
		if err != nil {
			t.Fatalf("timeline middle: %v", err)
		}
		if len(tl.Before) != 1 || tl.Before[0].ID != firstID {
			t.Fatalf("expected first in before list, got %+v", tl.Before)
		}
		if len(tl.After) != 1 || tl.After[0].ID != lastID {
			t.Fatalf("expected last in after list, got %+v", tl.After)
		}
	})

	t.Run("format context returns specific query stage errors", func(t *testing.T) {
		t.Run("recent sessions error", func(t *testing.T) {
			s := newTestStore(t)
			_ = s.Close()
			if _, err := s.FormatContext("", ""); err == nil {
				t.Fatalf("expected format context to fail from recent sessions")
			}
		})

		t.Run("recent observations error", func(t *testing.T) {
			s := newTestStore(t)
			if err := s.CreateSession("s-ctx", "mnemo", "/tmp/mnemo"); err != nil {
				t.Fatalf("create session: %v", err)
			}
			if _, err := s.db.Exec("DROP TABLE observations"); err != nil {
				t.Fatalf("drop observations: %v", err)
			}
			if _, err := s.FormatContext("", ""); err == nil {
				t.Fatalf("expected format context to fail from recent observations")
			}
		})

		t.Run("recent prompts error", func(t *testing.T) {
			s := newTestStore(t)
			if err := s.CreateSession("s-ctx2", "mnemo", "/tmp/mnemo"); err != nil {
				t.Fatalf("create session: %v", err)
			}
			if _, err := s.db.Exec("DROP TABLE user_prompts"); err != nil {
				t.Fatalf("drop prompts: %v", err)
			}
			if _, err := s.FormatContext("", ""); err == nil {
				t.Fatalf("expected format context to fail from recent prompts")
			}
		})
	})
}

func TestStoreUncoveredBranchesPushToHundred(t *testing.T) {
	t.Run("new open database hook error", func(t *testing.T) {
		orig := openDB
		t.Cleanup(func() { openDB = orig })
		openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
			return nil, errors.New("forced open error")
		}

		cfg := mustDefaultConfig(t)
		cfg.DataDir = t.TempDir()
		if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "open database") {
			t.Fatalf("expected open database error, got %v", err)
		}
	})

	t.Run("migrate forced failures for remaining exec branches", func(t *testing.T) {
		failCases := []string{
			"CREATE INDEX IF NOT EXISTS idx_obs_scope",
			"UPDATE observations SET topic_key = NULL",
			"UPDATE observations SET revision_count = 1",
			"UPDATE observations SET duplicate_count = 1",
			"UPDATE observations SET updated_at = created_at",
			"UPDATE user_prompts SET project = ''",
			"CREATE TRIGGER prompt_fts_insert",
		}
		for _, needle := range failCases {
			t.Run(needle, func(t *testing.T) {
				s := newTestStore(t)
				if strings.Contains(needle, "CREATE TRIGGER prompt_fts_insert") {
					if _, err := s.db.Exec(`
						DROP TRIGGER IF EXISTS prompt_fts_insert;
						DROP TRIGGER IF EXISTS prompt_fts_update;
						DROP TRIGGER IF EXISTS prompt_fts_delete;
					`); err != nil {
						t.Fatalf("drop prompt triggers: %v", err)
					}
				}
				origExec := s.hooks.exec
				s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
					if strings.Contains(query, needle) {
						return nil, errors.New("forced migrate failure")
					}
					return origExec(db, query, args...)
				}
				if err := s.migrate(); err == nil {
					t.Fatalf("expected migrate error for %q", needle)
				}
			})
		}
	})

	t.Run("migrate addColumn and legacy-call propagation", func(t *testing.T) {
		t.Run("propagates addColumn error", func(t *testing.T) {
			s := newTestStore(t)
			origQueryIt := s.hooks.queryIt
			called := 0
			s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
				if strings.Contains(query, "PRAGMA table_info(observations)") {
					called++
					if called == 1 {
						return nil, errors.New("forced addColumn failure")
					}
				}
				return origQueryIt(db, query, args...)
			}
			if err := s.migrate(); err == nil {
				t.Fatalf("expected migrate to propagate addColumn failure")
			}
		})

		t.Run("propagates legacy migrate error", func(t *testing.T) {
			s := newTestStore(t)
			origQueryIt := s.hooks.queryIt
			called := 0
			s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
				if strings.Contains(query, "PRAGMA table_info(observations)") {
					called++
					if called == 9 {
						return nil, errors.New("forced legacy call failure")
					}
				}
				return origQueryIt(db, query, args...)
			}
			if err := s.migrate(); err == nil {
				t.Fatalf("expected migrate to propagate legacy migrate failure")
			}
		})
	})

	t.Run("add observation, prompt, update forced errors", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.CreateSession("s-e", "mnemo", "/tmp/mnemo"); err != nil {
			t.Fatalf("create session: %v", err)
		}

		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-e", Type: "note", Title: "top", Content: "x", Project: "mnemo", TopicKey: "x"}); err != nil {
			t.Fatalf("seed topic observation: %v", err)
		}
		origExec := s.hooks.exec
		s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
			if strings.Contains(query, "SET type = ?") {
				return nil, errors.New("forced topic update error")
			}
			return origExec(db, query, args...)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-e", Type: "note", Title: "top", Content: "x", Project: "mnemo", TopicKey: "x"}); err == nil {
			t.Fatalf("expected topic upsert exec error")
		}

		s.hooks = defaultStoreHooks()
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-e", Type: "note", Title: "dup", Content: "dup content", Project: "mnemo"}); err != nil {
			t.Fatalf("seed dedupe observation: %v", err)
		}
		origExec = s.hooks.exec
		s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
			if strings.Contains(query, "SET duplicate_count = duplicate_count + 1") {
				return nil, errors.New("forced dedupe update error")
			}
			return origExec(db, query, args...)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-e", Type: "note", Title: "dup", Content: "dup content", Project: "mnemo"}); err == nil {
			t.Fatalf("expected dedupe exec error")
		}

		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-e", Type: "note", Title: "x", Content: "y", Project: "mnemo", TopicKey: "t"}); err == nil {
			t.Fatalf("expected topic query error on closed db")
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-e", Type: "note", Title: "x", Content: "y", Project: "mnemo"}); err == nil {
			t.Fatalf("expected dedupe query error on closed db")
		}
		if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-e", Content: "x"}); err == nil {
			t.Fatalf("expected add prompt error on closed db")
		}
	})

	t.Run("update observation remaining branches", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.CreateSession("s-u", "mnemo", "/tmp/mnemo"); err != nil {
			t.Fatalf("create session: %v", err)
		}
		id, err := s.AddObservation(AddObservationParams{SessionID: "s-u", Type: "old", Title: "t", Content: "c", Project: "mnemo", TopicKey: "topic/key"})
		if err != nil {
			t.Fatalf("seed observation: %v", err)
		}

		if _, err := s.UpdateObservation(999999, UpdateObservationParams{}); err == nil {
			t.Fatalf("expected update missing observation error")
		}

		newType := "new-type"
		longContent := strings.Repeat("z", s.cfg.MaxObservationLength+50)
		if _, err := s.UpdateObservation(id, UpdateObservationParams{Type: &newType, Content: &longContent}); err != nil {
			t.Fatalf("update with type+truncation: %v", err)
		}

		origExec := s.hooks.exec
		s.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
			if strings.Contains(query, "UPDATE observations") {
				return nil, errors.New("forced update exec error")
			}
			return origExec(db, query, args...)
		}
		if _, err := s.UpdateObservation(id, UpdateObservationParams{}); err == nil {
			t.Fatalf("expected update exec error")
		}
	})

	t.Run("query iterator scan and rows.Err branches", func(t *testing.T) {
		s := newTestStore(t)
		origQueryIt := s.hooks.queryIt

		setScanErr := func(match string) {
			s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
				if strings.Contains(query, match) {
					return &fakeRows{next: []bool{true, false}, scanErr: errors.New("forced scan error")}, nil
				}
				return origQueryIt(db, query, args...)
			}
		}

		setRowsErr := func(match string) {
			s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
				if strings.Contains(query, match) {
					return &fakeRows{next: []bool{false}, err: errors.New("forced rows err")}, nil
				}
				return origQueryIt(db, query, args...)
			}
		}

		if err := s.CreateSession("s-iter", "mnemo", "/tmp/mnemo"); err != nil {
			t.Fatalf("create session: %v", err)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-iter", Type: "note", Title: "one", Content: "one", Project: "mnemo"}); err != nil {
			t.Fatalf("add observation: %v", err)
		}
		if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-iter", Content: "prompt", Project: "mnemo"}); err != nil {
			t.Fatalf("add prompt: %v", err)
		}

		setScanErr("FROM sessions s")
		if _, err := s.RecentSessions("", 10); err == nil {
			t.Fatalf("expected recent sessions scan error")
		}

		setScanErr("FROM sessions s")
		if _, err := s.AllSessions("", 10); err == nil {
			t.Fatalf("expected all sessions scan error")
		}

		setScanErr("FROM user_prompts")
		if _, err := s.RecentPrompts("", 10); err == nil {
			t.Fatalf("expected recent prompts scan error")
		}

		setScanErr("FROM prompts_fts")
		if _, err := s.SearchPrompts("prompt", "", 10); err == nil {
			t.Fatalf("expected search prompts scan error")
		}

		setScanErr("FROM observations_fts")
		if _, err := s.Search("one", SearchOptions{}); err == nil {
			t.Fatalf("expected search scan error")
		}

		setRowsErr("FROM observations_fts")
		if _, err := s.Search("one", SearchOptions{}); err == nil {
			t.Fatalf("expected search rows err")
		}

		setScanErr("SELECT id, project, directory")
		if _, err := s.Export(); err == nil {
			t.Fatalf("expected export sessions scan error")
		}

		setRowsErr("SELECT id, project, directory")
		if _, err := s.Export(); err == nil {
			t.Fatalf("expected export sessions rows err")
		}

		setScanErr("FROM observations ORDER BY id")
		if _, err := s.Export(); err == nil {
			t.Fatalf("expected export observations scan error")
		}

		setRowsErr("FROM observations ORDER BY id")
		if _, err := s.Export(); err == nil {
			t.Fatalf("expected export observations rows err")
		}

		setScanErr("FROM user_prompts ORDER BY id")
		if _, err := s.Export(); err == nil {
			t.Fatalf("expected export prompts scan error")
		}

		setRowsErr("FROM user_prompts ORDER BY id")
		if _, err := s.Export(); err == nil {
			t.Fatalf("expected export prompts rows err")
		}

		setScanErr("FROM sync_chunks")
		if _, err := s.GetSyncedChunks(); err == nil {
			t.Fatalf("expected synced chunks scan error")
		}

		setRowsErr("PRAGMA table_info(extra_table)")
		if _, err := s.db.Exec(`CREATE TABLE extra_table (id INTEGER)`); err != nil {
			t.Fatalf("create extra table: %v", err)
		}
		if err := s.addColumnIfNotExists("extra_table", "n", "TEXT"); err == nil {
			t.Fatalf("expected add column rows err")
		}

		setScanErr("PRAGMA table_info(extra_table)")
		if err := s.addColumnIfNotExists("extra_table", "n2", "TEXT"); err == nil {
			t.Fatalf("expected add column scan error")
		}

		setRowsErr("PRAGMA table_info(observations)")
		if err := s.migrateLegacyObservationsTable(); err == nil {
			t.Fatalf("expected legacy migrate pragma rows err")
		}

		setScanErr("PRAGMA table_info(observations)")
		if err := s.migrateLegacyObservationsTable(); err == nil {
			t.Fatalf("expected legacy migrate pragma scan error")
		}

		s.hooks.queryIt = origQueryIt
	})

	t.Run("timeline and search type filter branches", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.CreateSession("s-t2", "mnemo", "/tmp/mnemo"); err != nil {
			t.Fatalf("create session: %v", err)
		}
		first, _ := s.AddObservation(AddObservationParams{SessionID: "s-t2", Type: "decision", Title: "a", Content: "a", Project: "mnemo"})
		_, _ = s.AddObservation(AddObservationParams{SessionID: "s-t2", Type: "decision", Title: "aa", Content: "aa", Project: "mnemo"})
		focus, _ := s.AddObservation(AddObservationParams{SessionID: "s-t2", Type: "decision", Title: "b", Content: "b", Project: "mnemo"})
		_, _ = s.AddObservation(AddObservationParams{SessionID: "s-t2", Type: "decision", Title: "c", Content: "c", Project: "mnemo"})

		if _, err := s.Search("b", SearchOptions{Type: "decision", Project: "mnemo", Scope: "project", Limit: 5}); err != nil {
			t.Fatalf("search with type filter: %v", err)
		}

		origQueryIt := s.hooks.queryIt
		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "id < ?") {
				return nil, errors.New("forced before query error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Timeline(focus, 2, 2); err == nil {
			t.Fatalf("expected timeline before query error")
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "id < ?") {
				return &fakeRows{next: []bool{true, false}, scanErr: errors.New("forced before scan error")}, nil
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Timeline(focus, 2, 2); err == nil {
			t.Fatalf("expected timeline before scan error")
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "id < ?") {
				return &fakeRows{next: []bool{false}, err: errors.New("forced before rows err")}, nil
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Timeline(focus, 2, 2); err == nil {
			t.Fatalf("expected timeline before rows err")
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "id > ?") {
				return nil, errors.New("forced after query error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Timeline(focus, 2, 2); err == nil {
			t.Fatalf("expected timeline after query error")
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "id > ?") {
				return &fakeRows{next: []bool{true, false}, scanErr: errors.New("forced after scan error")}, nil
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Timeline(focus, 2, 2); err == nil {
			t.Fatalf("expected timeline after scan error")
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "id > ?") {
				return &fakeRows{next: []bool{false}, err: errors.New("forced after rows err")}, nil
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Timeline(focus, 2, 2); err == nil {
			t.Fatalf("expected timeline after rows err")
		}

		s.hooks.queryIt = origQueryIt
		tl, err := s.Timeline(first, 5, 5)
		if err != nil {
			t.Fatalf("timeline reverse branch run: %v", err)
		}
		if len(tl.After) == 0 {
			t.Fatalf("expected timeline after entries")
		}
	})

	t.Run("format context and stats remaining branches", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.CreateSession("s-c", "mnemo", "/tmp/mnemo"); err != nil {
			t.Fatalf("create session: %v", err)
		}
		if _, err := s.AddObservation(AddObservationParams{SessionID: "s-c", Type: "note", Title: "n", Content: "n", Project: "mnemo"}); err != nil {
			t.Fatalf("add obs: %v", err)
		}

		origQueryIt := s.hooks.queryIt
		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "FROM observations o") && strings.Contains(query, "WHERE o.deleted_at IS NULL") {
				return nil, errors.New("forced recent observations error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.FormatContext("mnemo", "project"); err == nil {
			t.Fatalf("expected format context observations error")
		}

		s.hooks.queryIt = func(db queryer, query string, args ...any) (rowScanner, error) {
			if strings.Contains(query, "GROUP BY project") {
				return nil, errors.New("forced stats query error")
			}
			return origQueryIt(db, query, args...)
		}
		if _, err := s.Stats(); err != nil {
			t.Fatalf("stats should swallow project query errors: %v", err)
		}

		if err := s.EndSession("s-c", "has summary"); err != nil {
			t.Fatalf("end session: %v", err)
		}
		s.hooks.queryIt = origQueryIt
		ctx, err := s.FormatContext("mnemo", "project")
		if err != nil {
			t.Fatalf("format context with summary: %v", err)
		}
		if !strings.Contains(ctx, "has summary") {
			t.Fatalf("expected session summary included in context")
		}
	})

	t.Run("helper query errors and legacy migration late-stage failures", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
		if _, err := s.GetSyncedChunks(); err == nil {
			t.Fatalf("expected synced chunks query error")
		}
		if _, err := s.queryObservations("SELECT id FROM observations"); err == nil {
			t.Fatalf("expected queryObservations query error")
		}
		if err := s.addColumnIfNotExists("observations", "x", "TEXT"); err == nil {
			t.Fatalf("expected addColumn query error")
		}
		if err := s.migrateLegacyObservationsTable(); err == nil {
			t.Fatalf("expected legacy migrate query error")
		}

		s2 := newTestStore(t)
		if _, err := s2.db.Exec(`
			DROP TRIGGER IF EXISTS obs_fts_insert;
			DROP TRIGGER IF EXISTS obs_fts_update;
			DROP TRIGGER IF EXISTS obs_fts_delete;
			DROP TABLE IF EXISTS observations_fts;
			DROP TABLE observations;
			INSERT OR IGNORE INTO sessions (id, project, directory) VALUES ('s1', 'mnemo', '/tmp/mnemo');
			CREATE TABLE observations (
				id INT,
				session_id TEXT,
				type TEXT,
				title TEXT,
				content TEXT,
				tool_name TEXT,
				project TEXT,
				scope TEXT,
				topic_key TEXT,
				normalized_hash TEXT,
				revision_count INTEGER,
				duplicate_count INTEGER,
				last_seen_at TEXT,
				created_at TEXT,
				updated_at TEXT,
				deleted_at TEXT
			);
			INSERT INTO observations (id, session_id, type, title, content, project, created_at, updated_at)
			VALUES (1, 's1', 'bugfix', 'legacy', 'legacy row', 'mnemo', datetime('now'), datetime('now'));
		`); err != nil {
			t.Fatalf("prepare legacy table: %v", err)
		}

		lateFail := []string{"INSERT INTO observations_migrated", "DROP TABLE observations", "RENAME TO observations", "CREATE VIRTUAL TABLE observations_fts"}
		for _, needle := range lateFail {
			t.Run(needle, func(t *testing.T) {
				s3 := newTestStore(t)
				if _, err := s3.db.Exec(`
					DROP TRIGGER IF EXISTS obs_fts_insert;
					DROP TRIGGER IF EXISTS obs_fts_update;
					DROP TRIGGER IF EXISTS obs_fts_delete;
					DROP TABLE IF EXISTS observations_fts;
					DROP TABLE observations;
					INSERT OR IGNORE INTO sessions (id, project, directory) VALUES ('s1', 'mnemo', '/tmp/mnemo');
					CREATE TABLE observations (
						id INT,
						session_id TEXT,
						type TEXT,
						title TEXT,
						content TEXT,
						tool_name TEXT,
						project TEXT,
						scope TEXT,
						topic_key TEXT,
						normalized_hash TEXT,
						revision_count INTEGER,
						duplicate_count INTEGER,
						last_seen_at TEXT,
						created_at TEXT,
						updated_at TEXT,
						deleted_at TEXT
					);
					INSERT INTO observations (id, session_id, type, title, content, project, created_at, updated_at)
					VALUES (1, 's1', 'bugfix', 'legacy', 'legacy row', 'mnemo', datetime('now'), datetime('now'));
				`); err != nil {
					t.Fatalf("prepare legacy schema: %v", err)
				}

				origExec := s3.hooks.exec
				s3.hooks.exec = func(db execer, query string, args ...any) (sql.Result, error) {
					if strings.Contains(query, needle) {
						return nil, errors.New("forced legacy late failure")
					}
					return origExec(db, query, args...)
				}
				if err := s3.migrateLegacyObservationsTable(); err == nil {
					t.Fatalf("expected legacy migrate error for %q", needle)
				}
			})
		}
	})
}

// ─── Issue #25: Session collision regression tests ──────────────────────────

func TestCreateSessionUpsertsEmptyProjectAndDirectory(t *testing.T) {
	s := newTestStore(t)

	// Create session with empty project/directory (simulates first MCP call without context)
	if err := s.CreateSession("sess-upsert", "", ""); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Second call with real project/directory should fill in the blanks
	if err := s.CreateSession("sess-upsert", "projectA", "/tmp/a"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	sess, err := s.GetSession("sess-upsert")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Project != "projectA" {
		t.Fatalf("expected project=projectA after upsert, got %q", sess.Project)
	}
	if sess.Directory != "/tmp/a" {
		t.Fatalf("expected directory=/tmp/a after upsert, got %q", sess.Directory)
	}
}

func TestCreateSessionDoesNotOverwriteExistingProject(t *testing.T) {
	s := newTestStore(t)

	// Create session with project A
	if err := s.CreateSession("sess-preserve", "projectA", "/tmp/a"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Second call with project B should NOT overwrite
	if err := s.CreateSession("sess-preserve", "projectB", "/tmp/b"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	sess, err := s.GetSession("sess-preserve")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Project != "projectA" {
		t.Fatalf("expected project=projectA (preserved), got %q", sess.Project)
	}
	if sess.Directory != "/tmp/a" {
		t.Fatalf("expected directory=/tmp/a (preserved), got %q", sess.Directory)
	}
}

func TestCreateSessionPartialUpsert(t *testing.T) {
	s := newTestStore(t)

	t.Run("fills directory when project already set", func(t *testing.T) {
		if err := s.CreateSession("sess-partial-1", "myproject", ""); err != nil {
			t.Fatalf("create: %v", err)
		}
		// Second call fills directory but project stays
		if err := s.CreateSession("sess-partial-1", "other", "/new/dir"); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		sess, err := s.GetSession("sess-partial-1")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if sess.Project != "myproject" {
			t.Fatalf("project should be preserved, got %q", sess.Project)
		}
		if sess.Directory != "/new/dir" {
			t.Fatalf("directory should be filled, got %q", sess.Directory)
		}
	})

	t.Run("fills project when directory already set", func(t *testing.T) {
		if err := s.CreateSession("sess-partial-2", "", "/existing/dir"); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := s.CreateSession("sess-partial-2", "newproject", ""); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		sess, err := s.GetSession("sess-partial-2")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if sess.Project != "newproject" {
			t.Fatalf("project should be filled, got %q", sess.Project)
		}
		if sess.Directory != "/existing/dir" {
			t.Fatalf("directory should be preserved, got %q", sess.Directory)
		}
	})

	t.Run("both empty stays empty", func(t *testing.T) {
		if err := s.CreateSession("sess-partial-3", "", ""); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := s.CreateSession("sess-partial-3", "", ""); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		sess, err := s.GetSession("sess-partial-3")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if sess.Project != "" {
			t.Fatalf("project should stay empty, got %q", sess.Project)
		}
		if sess.Directory != "" {
			t.Fatalf("directory should stay empty, got %q", sess.Directory)
		}
	})
}

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "short ascii", in: "abc", max: 10, want: "abc"},
		{name: "exact length", in: "hello", max: 5, want: "hello"},
		{name: "long ascii", in: "abcdef", max: 3, want: "abc..."},
		{name: "spanish accents", in: "Decisión de arquitectura", max: 8, want: "Decisión..."},
		{name: "emoji", in: "🐛🔧🚀✨🎉💡", max: 3, want: "🐛🔧🚀..."},
		{name: "mixed ascii and multibyte", in: "café☕latte", max: 5, want: "café☕..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.in, tc.max)
			if got != tc.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

// ─── Project Enrollment CRUD Tests ───────────────────────────────────────────

func TestEnrollProjectBasic(t *testing.T) {
	s := newTestStore(t)

	// Enroll a project.
	if err := s.EnrollProject("mnemo"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}

	// Verify it shows up in the list.
	projects, err := s.ListEnrolledProjects()
	if err != nil {
		t.Fatalf("list enrolled projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 enrolled project, got %d", len(projects))
	}
	if projects[0].Project != "mnemo" {
		t.Fatalf("expected project 'mnemo', got %q", projects[0].Project)
	}
	if projects[0].EnrolledAt == "" {
		t.Fatal("expected enrolled_at to be set")
	}

	// Verify IsProjectEnrolled returns true.
	enrolled, err := s.IsProjectEnrolled("mnemo")
	if err != nil {
		t.Fatalf("is project enrolled: %v", err)
	}
	if !enrolled {
		t.Fatal("expected project to be enrolled")
	}
}

func TestEnrollProjectIdempotent(t *testing.T) {
	s := newTestStore(t)

	// Enroll twice — should not error.
	if err := s.EnrollProject("mnemo"); err != nil {
		t.Fatalf("first enroll: %v", err)
	}
	if err := s.EnrollProject("mnemo"); err != nil {
		t.Fatalf("second enroll (idempotent): %v", err)
	}

	// Should still be exactly one row.
	projects, err := s.ListEnrolledProjects()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 enrolled project after double-enroll, got %d", len(projects))
	}
}

func TestEnrollProjectBackfillsHistoricalMutations(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.db.Exec(
		`INSERT INTO sessions (id, project, directory, ended_at, summary) VALUES (?, ?, ?, datetime('now'), ?)`,
		"legacy-session", "legacy-proj", "/tmp/legacy", "done",
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO observations (sync_id, session_id, type, title, content, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 1, datetime('now'), datetime('now'))`,
		"obs-legacy", "legacy-session", "decision", "Legacy obs", "Historical content", "legacy-proj", "project", hashNormalized("Historical content"),
	); err != nil {
		t.Fatalf("insert observation: %v", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO user_prompts (sync_id, session_id, content, project) VALUES (?, ?, ?, ?)`,
		"prompt-legacy", "legacy-session", "What happened before enterprise?", "legacy-proj",
	); err != nil {
		t.Fatalf("insert prompt: %v", err)
	}

	var before int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations`).Scan(&before); err != nil {
		t.Fatalf("count mutations before enroll: %v", err)
	}
	if before != 0 {
		t.Fatalf("expected 0 sync mutations before enroll, got %d", before)
	}

	if err := s.EnrollProject("legacy-proj"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(mutations) != 3 {
		t.Fatalf("expected 3 backfilled mutations, got %d", len(mutations))
	}

	expected := map[string]string{
		SyncEntitySession:     "legacy-session",
		SyncEntityObservation: "obs-legacy",
		SyncEntityPrompt:      "prompt-legacy",
	}
	for _, mutation := range mutations {
		entityKey, ok := expected[mutation.Entity]
		if !ok {
			t.Fatalf("unexpected mutation entity %q", mutation.Entity)
		}
		if mutation.EntityKey != entityKey {
			t.Fatalf("expected entity_key %q for %s, got %q", entityKey, mutation.Entity, mutation.EntityKey)
		}
		if mutation.Project != "legacy-proj" {
			t.Fatalf("expected project legacy-proj, got %q", mutation.Project)
		}
	}
	state, err := s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("get sync state: %v", err)
	}
	if state.LastEnqueuedSeq != 3 {
		t.Fatalf("expected last_enqueued_seq 3 after backfill, got %d", state.LastEnqueuedSeq)
	}
}

func TestEnrollProjectBackfillIsIdempotentAndSkipsExistingMutations(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.db.Exec(
		`INSERT INTO sessions (id, project, directory) VALUES (?, ?, ?)`,
		"legacy-session", "legacy-proj", "/tmp/legacy",
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO observations (sync_id, session_id, type, title, content, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 1, datetime('now'), datetime('now'))`,
		"obs-legacy", "legacy-session", "decision", "Legacy obs", "Historical content", "legacy-proj", "project", hashNormalized("Historical content"),
	); err != nil {
		t.Fatalf("insert observation: %v", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO user_prompts (sync_id, session_id, content, project) VALUES (?, ?, ?, ?)`,
		"prompt-legacy", "legacy-session", "Historical prompt", "legacy-proj",
	); err != nil {
		t.Fatalf("insert prompt: %v", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source, project)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		DefaultSyncTargetKey, SyncEntityObservation, "obs-legacy", SyncOpUpsert, `{"sync_id":"obs-legacy","session_id":"legacy-session","project":"legacy-proj"}`, SyncSourceLocal, "legacy-proj",
	); err != nil {
		t.Fatalf("insert existing mutation: %v", err)
	}

	if err := s.EnrollProject("legacy-proj"); err != nil {
		t.Fatalf("first enroll: %v", err)
	}

	var afterFirst int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations`).Scan(&afterFirst); err != nil {
		t.Fatalf("count after first enroll: %v", err)
	}
	if afterFirst != 3 {
		t.Fatalf("expected 3 total mutations after first enroll, got %d", afterFirst)
	}

	var observationMutations int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE entity = ? AND entity_key = ?`, SyncEntityObservation, "obs-legacy").Scan(&observationMutations); err != nil {
		t.Fatalf("count observation mutations: %v", err)
	}
	if observationMutations != 1 {
		t.Fatalf("expected existing observation mutation to remain single, got %d rows", observationMutations)
	}

	if err := s.EnrollProject("legacy-proj"); err != nil {
		t.Fatalf("second enroll: %v", err)
	}

	var afterSecond int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations`).Scan(&afterSecond); err != nil {
		t.Fatalf("count after second enroll: %v", err)
	}
	if afterSecond != afterFirst {
		t.Fatalf("expected no duplicate backfill on re-enroll, got %d mutations after second enroll vs %d after first", afterSecond, afterFirst)
	}
}

func TestNewRepairsAlreadyEnrolledProjectsMissingHistoricalSyncMutations(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "memory.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	obsHash := hashNormalized("Historical content")
	_, err = db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			directory TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			summary TEXT
		);
		CREATE TABLE observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sync_id TEXT,
			session_id TEXT NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tool_name TEXT,
			project TEXT,
			scope TEXT NOT NULL DEFAULT 'project',
			topic_key TEXT,
			normalized_hash TEXT,
			revision_count INTEGER NOT NULL DEFAULT 1,
			duplicate_count INTEGER NOT NULL DEFAULT 1,
			last_seen_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			deleted_at TEXT,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
		CREATE TABLE user_prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sync_id TEXT,
			session_id TEXT NOT NULL,
			content TEXT NOT NULL,
			project TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
		CREATE TABLE sync_state (
			target_key TEXT PRIMARY KEY,
			lifecycle TEXT NOT NULL DEFAULT 'idle',
			last_enqueued_seq INTEGER NOT NULL DEFAULT 0,
			last_acked_seq INTEGER NOT NULL DEFAULT 0,
			last_pulled_seq INTEGER NOT NULL DEFAULT 0,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			backoff_until TEXT,
			lease_owner TEXT,
			lease_until TEXT,
			last_error TEXT,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE sync_mutations (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			target_key TEXT NOT NULL,
			entity TEXT NOT NULL,
			entity_key TEXT NOT NULL,
			op TEXT NOT NULL,
			payload TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'local',
			occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
			acked_at TEXT,
			project TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (target_key) REFERENCES sync_state(target_key)
		);
		CREATE TABLE sync_enrolled_projects (
			project TEXT PRIMARY KEY,
			enrolled_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO sessions (id, project, directory, summary) VALUES ('legacy-session', 'legacy-proj', '/tmp/legacy', 'done');
		INSERT INTO observations (sync_id, session_id, type, title, content, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, updated_at)
		VALUES ('obs-legacy', 'legacy-session', 'decision', 'Legacy obs', 'Historical content', 'legacy-proj', 'project', ?, 1, 1, datetime('now'), datetime('now'));
		INSERT INTO user_prompts (sync_id, session_id, content, project) VALUES ('prompt-legacy', 'legacy-session', 'Historical prompt', 'legacy-proj');
		INSERT INTO sync_state (target_key, lifecycle, updated_at) VALUES (?, 'idle', datetime('now'));
		INSERT INTO sync_enrolled_projects (project) VALUES ('legacy-proj');
	`, obsHash, DefaultSyncTargetKey)
	if err != nil {
		_ = db.Close()
		t.Fatalf("seed legacy db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	cfg := mustDefaultConfig(t)
	cfg.DataDir = dataDir

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("new store after enrolled legacy state: %v", err)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 10)
	if err != nil {
		_ = s.Close()
		t.Fatalf("list pending after repair: %v", err)
	}
	if len(mutations) != 3 {
		_ = s.Close()
		t.Fatalf("expected 3 repaired mutations, got %d", len(mutations))
	}

	state, err := s.GetSyncState(DefaultSyncTargetKey)
	if err != nil {
		_ = s.Close()
		t.Fatalf("get sync state after repair: %v", err)
	}
	if state.LastEnqueuedSeq != 3 {
		_ = s.Close()
		t.Fatalf("expected last_enqueued_seq 3 after automatic repair, got %d", state.LastEnqueuedSeq)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close repaired store: %v", err)
	}

	s, err = New(cfg)
	if err != nil {
		t.Fatalf("reopen repaired store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations`).Scan(&count); err != nil {
		t.Fatalf("count repaired mutations after reopen: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected repair to stay idempotent across reopen, got %d sync mutations", count)
	}
}

func TestEnrollProjectEmptyNameReturnsError(t *testing.T) {
	s := newTestStore(t)

	if err := s.EnrollProject(""); err == nil {
		t.Fatal("expected error when enrolling empty project name")
	}
}

func TestUnenrollProjectBasic(t *testing.T) {
	s := newTestStore(t)

	if err := s.EnrollProject("mnemo"); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// Unenroll.
	if err := s.UnenrollProject("mnemo"); err != nil {
		t.Fatalf("unenroll: %v", err)
	}

	// Should be gone.
	enrolled, err := s.IsProjectEnrolled("mnemo")
	if err != nil {
		t.Fatalf("is enrolled after unenroll: %v", err)
	}
	if enrolled {
		t.Fatal("expected project to be unenrolled")
	}

	projects, err := s.ListEnrolledProjects()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 enrolled projects after unenroll, got %d", len(projects))
	}
}

func TestUnenrollProjectIdempotent(t *testing.T) {
	s := newTestStore(t)

	// Unenroll a project that was never enrolled — should not error.
	if err := s.UnenrollProject("nonexistent"); err != nil {
		t.Fatalf("unenroll non-enrolled project should be idempotent: %v", err)
	}
}

func TestUnenrollProjectEmptyNameReturnsError(t *testing.T) {
	s := newTestStore(t)

	if err := s.UnenrollProject(""); err == nil {
		t.Fatal("expected error when unenrolling empty project name")
	}
}

func TestIsProjectEnrolledReturnsFalseForUnknown(t *testing.T) {
	s := newTestStore(t)

	enrolled, err := s.IsProjectEnrolled("unknown-project")
	if err != nil {
		t.Fatalf("is enrolled: %v", err)
	}
	if enrolled {
		t.Fatal("expected false for unknown project")
	}
}

func TestListEnrolledProjectsEmpty(t *testing.T) {
	s := newTestStore(t)

	projects, err := s.ListEnrolledProjects()
	if err != nil {
		t.Fatalf("list enrolled projects: %v", err)
	}
	if projects != nil {
		t.Fatalf("expected nil for empty list, got %v", projects)
	}
}

func TestListEnrolledProjectsAlphabeticalOrder(t *testing.T) {
	s := newTestStore(t)

	// Enroll in non-alphabetical order.
	for _, p := range []string{"zebra", "alpha", "mango"} {
		if err := s.EnrollProject(p); err != nil {
			t.Fatalf("enroll %q: %v", p, err)
		}
	}

	projects, err := s.ListEnrolledProjects()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
	expected := []string{"alpha", "mango", "zebra"}
	for i, ep := range projects {
		if ep.Project != expected[i] {
			t.Fatalf("position %d: expected %q, got %q", i, expected[i], ep.Project)
		}
	}
}

func TestSyncMutationProjectColumnExists(t *testing.T) {
	s := newTestStore(t)

	// Verify the project column exists on sync_mutations by inserting a row.
	_, err := s.db.Exec(
		`INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source, project)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		DefaultSyncTargetKey, "session", "test-key", SyncOpUpsert, `{"project":"myproj"}`, SyncSourceLocal, "myproj",
	)
	if err != nil {
		t.Fatalf("insert sync_mutation with project: %v", err)
	}

	// Read it back and verify project is populated.
	var project string
	if err := s.db.QueryRow(`SELECT project FROM sync_mutations WHERE entity_key = ?`, "test-key").Scan(&project); err != nil {
		t.Fatalf("scan project: %v", err)
	}
	if project != "myproj" {
		t.Fatalf("expected project 'myproj', got %q", project)
	}
}

func TestSyncMutationProjectBackfill(t *testing.T) {
	s := newTestStore(t)

	// Insert a mutation that simulates a pre-migration row (project is empty, but payload has it).
	// The backfill runs during schema init, so we test it by inserting directly then re-running.
	// Since the store already ran migrations, let's verify backfill logic by inserting a new row
	// with empty project and manually running the backfill.
	_, err := s.db.Exec(
		`INSERT INTO sync_mutations (target_key, entity, entity_key, op, payload, source, project)
		 VALUES (?, ?, ?, ?, ?, ?, '')`,
		DefaultSyncTargetKey, "observation", "backfill-key", SyncOpUpsert, `{"project":"backfilled"}`, SyncSourceLocal,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Run the backfill manually.
	_, err = s.db.Exec(`
		UPDATE sync_mutations
		SET project = COALESCE(json_extract(payload, '$.project'), '')
		WHERE project = '' AND payload != ''
	`)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}

	var project string
	if err := s.db.QueryRow(`SELECT project FROM sync_mutations WHERE entity_key = ?`, "backfill-key").Scan(&project); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if project != "backfilled" {
		t.Fatalf("expected backfilled project 'backfilled', got %q", project)
	}
}

func TestListPendingSyncMutationsIncludesProject(t *testing.T) {
	s := newTestStore(t)

	// Enroll the project so mutations are visible in ListPendingSyncMutations.
	if err := s.EnrollProject("my-project"); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if err := s.CreateSession("proj-session", "my-project", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := s.AddObservation(AddObservationParams{
		SessionID: "proj-session",
		Type:      "decision",
		Title:     "Test obs",
		Content:   "Content",
		Project:   "my-project",
		Scope:     "project",
	})
	if err != nil {
		t.Fatalf("add observation: %v", err)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}

	// There should be mutations (session create + observation create at minimum).
	if len(mutations) == 0 {
		t.Fatal("expected at least one pending mutation")
	}

	// Phase 3: Verify the Project field is populated at enqueue time.
	foundProject := false
	for _, m := range mutations {
		if m.Project == "my-project" {
			foundProject = true
			break
		}
	}
	if !foundProject {
		t.Fatal("expected at least one mutation with project='my-project'")
	}
}

// ─── Phase 3: extractProjectFromPayload ──────────────────────────────────────

func TestExtractProjectFromSessionPayload(t *testing.T) {
	p := syncSessionPayload{ID: "s1", Project: "acme"}
	got := extractProjectFromPayload(p)
	if got != "acme" {
		t.Fatalf("expected 'acme', got %q", got)
	}
}

func TestExtractProjectFromObservationPayload(t *testing.T) {
	proj := "obs-project"
	p := syncObservationPayload{SyncID: "obs-1", Project: &proj}
	got := extractProjectFromPayload(p)
	if got != "obs-project" {
		t.Fatalf("expected 'obs-project', got %q", got)
	}
}

func TestExtractProjectFromObservationPayloadNil(t *testing.T) {
	p := syncObservationPayload{SyncID: "obs-1", Project: nil}
	got := extractProjectFromPayload(p)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtractProjectFromPromptPayload(t *testing.T) {
	proj := "prompt-project"
	p := syncPromptPayload{SyncID: "p1", Project: &proj}
	got := extractProjectFromPayload(p)
	if got != "prompt-project" {
		t.Fatalf("expected 'prompt-project', got %q", got)
	}
}

func TestExtractProjectFromPromptPayloadNil(t *testing.T) {
	p := syncPromptPayload{SyncID: "p1", Project: nil}
	got := extractProjectFromPayload(p)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtractProjectFromUnknownPayloadFallback(t *testing.T) {
	// Unknown struct with a project field — uses JSON fallback.
	p := struct {
		Project string `json:"project"`
		Other   string `json:"other"`
	}{Project: "fallback-proj", Other: "x"}
	got := extractProjectFromPayload(p)
	if got != "fallback-proj" {
		t.Fatalf("expected 'fallback-proj', got %q", got)
	}
}

func TestExtractProjectFromPayloadWithoutProjectField(t *testing.T) {
	// Unknown struct without a project field — returns empty.
	p := struct {
		Name string `json:"name"`
	}{Name: "test"}
	got := extractProjectFromPayload(p)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

// ─── Phase 3: enqueueSyncMutationTx populates project column ────────────────

func TestEnqueueSyncMutationPopulatesProjectFromSessionPayload(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("enq-session", "enqueued-project", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// CreateSession enqueues a sync mutation internally. Check the project column.
	var project string
	err := s.db.QueryRow(
		`SELECT project FROM sync_mutations WHERE entity = ? AND entity_key = ?`,
		SyncEntitySession, "enq-session",
	).Scan(&project)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if project != "enqueued-project" {
		t.Fatalf("expected project='enqueued-project', got %q", project)
	}
}

func TestEnqueueSyncMutationPopulatesProjectFromObservationPayload(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("obs-enq", "obs-proj", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := s.AddObservation(AddObservationParams{
		SessionID: "obs-enq",
		Type:      "decision",
		Title:     "Test",
		Content:   "Content",
		Project:   "obs-proj",
	})
	if err != nil {
		t.Fatalf("add observation: %v", err)
	}

	// Check the observation mutation's project column.
	var project string
	err = s.db.QueryRow(
		`SELECT project FROM sync_mutations WHERE entity = ? ORDER BY seq DESC LIMIT 1`,
		SyncEntityObservation,
	).Scan(&project)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if project != "obs-proj" {
		t.Fatalf("expected project='obs-proj', got %q", project)
	}
}

func TestEnqueueSyncMutationPopulatesProjectFromPromptPayload(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("prompt-enq", "prompt-proj", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := s.AddPrompt(AddPromptParams{
		SessionID: "prompt-enq",
		Content:   "What did we do?",
		Project:   "prompt-proj",
	})
	if err != nil {
		t.Fatalf("add prompt: %v", err)
	}

	var project string
	err = s.db.QueryRow(
		`SELECT project FROM sync_mutations WHERE entity = ? ORDER BY seq DESC LIMIT 1`,
		SyncEntityPrompt,
	).Scan(&project)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if project != "prompt-proj" {
		t.Fatalf("expected project='prompt-proj', got %q", project)
	}
}

// ─── Phase 4: ListPendingSyncMutations enrollment filtering ──────────────────

func TestListPendingFiltersNonEnrolledProjects(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-enrolled", "enrolled-proj", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := s.CreateSession("s-not-enrolled", "other-proj", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Enroll only "enrolled-proj".
	if err := s.EnrollProject("enrolled-proj"); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}

	// Only enrolled-proj mutations should appear.
	for _, m := range mutations {
		if m.Project == "other-proj" {
			t.Fatalf("non-enrolled project 'other-proj' should not appear in pending mutations")
		}
	}

	foundEnrolled := false
	for _, m := range mutations {
		if m.Project == "enrolled-proj" {
			foundEnrolled = true
			break
		}
	}
	if !foundEnrolled {
		t.Fatal("expected enrolled-proj mutations to appear")
	}
}

func TestListPendingReturnsNoMutationsWhenNoneEnrolled(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-no-enroll", "some-proj", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}

	// No projects enrolled → no mutations (all have project != '').
	if len(mutations) != 0 {
		t.Fatalf("expected 0 mutations when no projects enrolled, got %d", len(mutations))
	}
}

// ─── Phase 4: SkipAckNonEnrolledMutations ────────────────────────────────────

func TestSkipAckNonEnrolledMutationsBasic(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("skip-session", "skip-proj", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Do NOT enroll "skip-proj" → mutations should be skip-acked.
	skipped, err := s.SkipAckNonEnrolledMutations(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("skip-ack: %v", err)
	}
	if skipped == 0 {
		t.Fatal("expected at least one mutation to be skip-acked")
	}

	// After skip-ack, there should be no pending mutations left.
	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(mutations) != 0 {
		t.Fatalf("expected 0 pending mutations after skip-ack, got %d", len(mutations))
	}
}

func TestSkipAckPreservesEnrolledProjectMutations(t *testing.T) {
	s := newTestStore(t)

	if err := s.EnrollProject("enrolled"); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if err := s.CreateSession("s-enrolled", "enrolled", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := s.CreateSession("s-not-enrolled", "not-enrolled", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Count total pending before skip-ack.
	var totalBefore int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE acked_at IS NULL`).Scan(&totalBefore)

	skipped, err := s.SkipAckNonEnrolledMutations(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("skip-ack: %v", err)
	}
	if skipped == 0 {
		t.Fatal("expected at least one mutation to be skip-acked for 'not-enrolled'")
	}

	// Remaining pending should be only "enrolled" mutations.
	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	for _, m := range mutations {
		if m.Project == "not-enrolled" {
			t.Fatal("skip-acked mutation still appears as pending")
		}
	}
	if len(mutations) == 0 {
		t.Fatal("expected enrolled-project mutations to remain")
	}
}

// ─── Phase 5: Empty/global project always syncs ──────────────────────────────

func TestEmptyProjectMutationsAlwaysSync(t *testing.T) {
	s := newTestStore(t)

	// Create a session with empty project (global).
	if err := s.CreateSession("global-session", "", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// No projects enrolled, but empty-project mutations should still appear.
	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}

	if len(mutations) == 0 {
		t.Fatal("expected empty-project mutations to always sync regardless of enrollment")
	}

	// Verify they have project = ''.
	for _, m := range mutations {
		if m.Project != "" {
			t.Fatalf("expected empty project, got %q", m.Project)
		}
	}
}

func TestSkipAckDoesNotAffectEmptyProjectMutations(t *testing.T) {
	s := newTestStore(t)

	// Create a session with empty project (global).
	if err := s.CreateSession("global-session-2", "", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Count pending before skip-ack.
	beforeMutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	beforeCount := len(beforeMutations)

	// Skip-ack should not affect empty-project mutations.
	skipped, err := s.SkipAckNonEnrolledMutations(DefaultSyncTargetKey)
	if err != nil {
		t.Fatalf("skip-ack: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 mutations to be skip-acked (all empty project), got %d", skipped)
	}

	// Verify count unchanged.
	afterMutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(afterMutations) != beforeCount {
		t.Fatalf("expected %d mutations after skip-ack, got %d", beforeCount, len(afterMutations))
	}
}

func TestMixedEnrolledAndEmptyProjectMutations(t *testing.T) {
	s := newTestStore(t)

	if err := s.EnrollProject("enrolled-mix"); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// Create sessions with different project states.
	if err := s.CreateSession("mix-enrolled", "enrolled-mix", "/tmp"); err != nil {
		t.Fatalf("create enrolled session: %v", err)
	}
	if err := s.CreateSession("mix-global", "", "/tmp"); err != nil {
		t.Fatalf("create global session: %v", err)
	}
	if err := s.CreateSession("mix-unenrolled", "unenrolled-mix", "/tmp"); err != nil {
		t.Fatalf("create unenrolled session: %v", err)
	}

	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}

	// Should have enrolled-mix and empty-project mutations, but NOT unenrolled-mix.
	var hasEnrolled, hasGlobal bool
	for _, m := range mutations {
		if m.Project == "unenrolled-mix" {
			t.Fatal("unenrolled project mutations should not appear")
		}
		if m.Project == "enrolled-mix" {
			hasEnrolled = true
		}
		if m.Project == "" {
			hasGlobal = true
		}
	}
	if !hasEnrolled {
		t.Fatal("expected enrolled-mix mutations to appear")
	}
	if !hasGlobal {
		t.Fatal("expected empty-project (global) mutations to appear")
	}
}

// ─── MigrateProject ─────────────────────────────────────────────────────────

func TestMigrateProject(t *testing.T) {
	s := newTestStore(t)
	old, new_ := "old-name", "new-name"

	// Seed data under old project name
	if err := s.CreateSession("s1", old, "/tmp/old"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "s1", Type: "decision", Title: "test obs",
		Content: "some content", Project: old, Scope: "project",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if _, err := s.AddPrompt(AddPromptParams{SessionID: "s1", Content: "test prompt", Project: old}); err != nil {
		t.Fatalf("AddPrompt: %v", err)
	}

	// Run migration
	result, err := s.MigrateProject(old, new_)
	if err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}
	if !result.Migrated {
		t.Fatal("expected migration to happen")
	}
	if result.ObservationsUpdated != 1 {
		t.Fatalf("expected 1 observation migrated, got %d", result.ObservationsUpdated)
	}
	if result.SessionsUpdated != 1 {
		t.Fatalf("expected 1 session migrated, got %d", result.SessionsUpdated)
	}
	if result.PromptsUpdated != 1 {
		t.Fatalf("expected 1 prompt migrated, got %d", result.PromptsUpdated)
	}

	// Verify old project has no records
	obs, _ := s.RecentObservations(old, "", 10)
	if len(obs) != 0 {
		t.Fatalf("expected 0 observations under old name, got %d", len(obs))
	}

	// Verify new project has the records
	obs, _ = s.RecentObservations(new_, "", 10)
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation under new name, got %d", len(obs))
	}

	// Verify FTS search finds it under new project
	results, _ := s.Search("test obs", SearchOptions{Project: new_, Limit: 10})
	if len(results) != 1 {
		t.Fatalf("expected FTS to find 1 result under new project, got %d", len(results))
	}
}

func TestMigrateProjectNoOp(t *testing.T) {
	s := newTestStore(t)

	// No records under "nonexistent" — should be a no-op
	result, err := s.MigrateProject("nonexistent", "anything")
	if err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}
	if result.Migrated {
		t.Fatal("expected no migration for nonexistent project")
	}
}

func TestMigrateProjectIdempotent(t *testing.T) {
	s := newTestStore(t)
	old, new_ := "old-proj", "new-proj"

	if err := s.CreateSession("s1", old, "/tmp"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "s1", Type: "decision", Title: "test",
		Content: "content", Project: old, Scope: "project",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	// First migration
	r1, err := s.MigrateProject(old, new_)
	if err != nil {
		t.Fatalf("first MigrateProject: %v", err)
	}
	if !r1.Migrated {
		t.Fatal("first migration should migrate")
	}

	// Second migration — no records under old name anymore
	r2, err := s.MigrateProject(old, new_)
	if err != nil {
		t.Fatalf("second MigrateProject: %v", err)
	}
	if r2.Migrated {
		t.Fatal("second migration should be a no-op")
	}
}


// ─── Tags ────────────────────────────────────────────────────────────────────

func newTestSession(t *testing.T, s *Store, id, project string) {
	t.Helper()
	if err := s.CreateSession(id, project, ""); err != nil {
		t.Fatalf("CreateSession(%q): %v", id, err)
	}
}

func TestAddObservationWithTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-tags-1", "mnemo")

	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-tags-1",
		Type:      "decision",
		Title:     "use sqlite for storage",
		Content:   "decided to use sqlite because it is embedded and requires no external process",
		Project:   "mnemo",
		Tags:      []string{"database", "architecture", "sqlite"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	if len(obs.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(obs.Tags), obs.Tags)
	}
	tagSet := make(map[string]bool)
	for _, tag := range obs.Tags {
		tagSet[tag] = true
	}
	// "database" is an alias for "db", so the stored tag is "db".
	for _, want := range []string{"db", "architecture", "sqlite"} {
		if !tagSet[want] {
			t.Errorf("missing tag %q, got: %v", want, obs.Tags)
		}
	}
}

func TestAddObservationTagNormalization(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-norm", "mnemo")

	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-norm",
		Type:      "decision",
		Title:     "normalization test",
		Content:   "tags should be normalized to lowercase with hyphens",
		Project:   "mnemo",
		Tags:      []string{"  Auth  ", "BACKEND", "some thing"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	tagSet := make(map[string]bool)
	for _, tag := range obs.Tags {
		tagSet[tag] = true
	}
	if !tagSet["auth"] {
		t.Errorf("expected normalized tag 'auth', got: %v", obs.Tags)
	}
	if !tagSet["backend"] {
		t.Errorf("expected normalized tag 'backend', got: %v", obs.Tags)
	}
	if !tagSet["some-thing"] {
		t.Errorf("expected normalized tag 'some-thing', got: %v", obs.Tags)
	}
}

func TestUpdateObservationReplaceTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-update-tags", "mnemo")

	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-update-tags",
		Type:      "decision",
		Title:     "original tags",
		Content:   "content with original tags",
		Project:   "mnemo",
		Tags:      []string{"alpha", "beta"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	newTags := []string{"gamma", "delta"}
	_, err = s.UpdateObservation(id, UpdateObservationParams{
		Tags: &newTags,
	})
	if err != nil {
		t.Fatalf("UpdateObservation: %v", err)
	}

	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	if len(obs.Tags) != 2 {
		t.Fatalf("expected 2 tags after update, got %d: %v", len(obs.Tags), obs.Tags)
	}
	tagSet := make(map[string]bool)
	for _, tag := range obs.Tags {
		tagSet[tag] = true
	}
	if tagSet["alpha"] || tagSet["beta"] {
		t.Error("old tags should have been replaced")
	}
	if !tagSet["gamma"] || !tagSet["delta"] {
		t.Errorf("new tags missing, got: %v", obs.Tags)
	}
}

func TestUpdateObservationClearTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-clear-tags", "mnemo")

	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-clear-tags",
		Type:      "decision",
		Title:     "tags to be cleared",
		Content:   "will have tags removed after update",
		Project:   "mnemo",
		Tags:      []string{"remove-me"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	empty := []string{}
	_, err = s.UpdateObservation(id, UpdateObservationParams{
		Tags: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateObservation: %v", err)
	}

	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	if len(obs.Tags) != 0 {
		t.Errorf("expected no tags after clear, got: %v", obs.Tags)
	}
}

func TestUpdateObservationOmittedTagsPreserved(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-preserve-tags", "mnemo")

	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-preserve-tags",
		Type:      "decision",
		Title:     "preserve tags",
		Content:   "tags should not change when Tags field is nil",
		Project:   "mnemo",
		Tags:      []string{"keep-me"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	newTitle := "updated title"
	_, err = s.UpdateObservation(id, UpdateObservationParams{
		Title: &newTitle,
		// Tags is nil — should not touch tags
	})
	if err != nil {
		t.Fatalf("UpdateObservation: %v", err)
	}

	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	if len(obs.Tags) != 1 || obs.Tags[0] != "keep-me" {
		t.Errorf("expected tags to be preserved, got: %v", obs.Tags)
	}
}

func TestSearchFilterByTags(t *testing.T) {
	s := newTestStore(t)
	sessID := "sess-search-tags"
	newTestSession(t, s, sessID, "mnemo")

	_, err := s.AddObservation(AddObservationParams{
		SessionID: sessID,
		Type:      "decision",
		Title:     "jwt authentication middleware",
		Content:   "jwt middleware implemented for authenticated routes",
		Project:   "mnemo",
		Tags:      []string{"auth", "backend"},
	})
	if err != nil {
		t.Fatalf("AddObservation auth: %v", err)
	}

	_, err = s.AddObservation(AddObservationParams{
		SessionID: sessID,
		Type:      "decision",
		Title:     "sqlite schema migrations",
		Content:   "sqlite schema migration system applied",
		Project:   "mnemo",
		Tags:      []string{"database", "backend"},
	})
	if err != nil {
		t.Fatalf("AddObservation db: %v", err)
	}

	// "auth" tag present only in first observation
	results, err := s.Search("jwt", SearchOptions{Tags: []string{"auth"}})
	if err != nil {
		t.Fatalf("Search with auth tag: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with tag 'auth', got %d", len(results))
	}
	if results[0].Title != "jwt authentication middleware" {
		t.Errorf("unexpected result title: %q", results[0].Title)
	}

	// "backend" tag present in both
	results, err = s.Search("decision", SearchOptions{Tags: []string{"backend"}})
	if err != nil {
		t.Fatalf("Search with backend tag: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results with tag 'backend', got %d", len(results))
	}
}

func TestSearchResultsHaveTags(t *testing.T) {
	s := newTestStore(t)
	sessID := "sess-search-tags-load"
	newTestSession(t, s, sessID, "mnemo")

	_, err := s.AddObservation(AddObservationParams{
		SessionID: sessID,
		Type:      "decision",
		Title:     "tagged memory observation",
		Content:   "observation with tags for search result verification purpose",
		Project:   "mnemo",
		Tags:      []string{"verify", "search"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	results, err := s.Search("tagged memory", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if len(results[0].Tags) == 0 {
		t.Error("search results should have tags loaded")
	}
}

func TestRecentObservationsFilterByTags(t *testing.T) {
	s := newTestStore(t)
	sessID := "sess-recent-tags"
	newTestSession(t, s, sessID, "mnemo")

	_, err := s.AddObservation(AddObservationParams{
		SessionID: sessID,
		Type:      "decision",
		Title:     "auth decision",
		Content:   "auth content details",
		Project:   "mnemo",
		Tags:      []string{"auth"},
	})
	if err != nil {
		t.Fatalf("AddObservation auth: %v", err)
	}

	_, err = s.AddObservation(AddObservationParams{
		SessionID: sessID,
		Type:      "decision",
		Title:     "perf decision",
		Content:   "performance content details",
		Project:   "mnemo",
		Tags:      []string{"performance"},
	})
	if err != nil {
		t.Fatalf("AddObservation perf: %v", err)
	}

	obs, err := s.RecentObservations("mnemo", "project", 10, "auth")
	if err != nil {
		t.Fatalf("RecentObservations: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation with tag 'auth', got %d", len(obs))
	}
	if obs[0].Title != "auth decision" {
		t.Errorf("unexpected observation: %q", obs[0].Title)
	}
}

func TestSuggestTags(t *testing.T) {
	cases := []struct {
		typ     string
		title   string
		content string
		wantTag string
	}{
		{"bugfix", "fix auth crash", "nil pointer in auth handler", "bug"},
		{"architecture", "database design", "chose sqlite over postgres", "architecture"},
		{"decision", "api versioning", "decided to use semver for api", "decision"},
		{"config", "ci pipeline", "set up github actions for ci", "config"},
	}

	for _, tc := range cases {
		tags := SuggestTags(tc.typ, tc.title, tc.content)
		if len(tags) == 0 {
			t.Errorf("SuggestTags(%q, %q, ...) returned no tags", tc.typ, tc.title)
			continue
		}
		found := false
		for _, tag := range tags {
			if tag == tc.wantTag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SuggestTags(%q, %q, ...) = %v, want tag %q", tc.typ, tc.title, tags, tc.wantTag)
		}
	}
}

func TestSuggestTagsNoBlockedTags(t *testing.T) {
	// Generic fallback ("topic") must not appear in suggestions.
	tags := SuggestTags("", "some observation", "general content without matching keywords")
	for _, tag := range tags {
		if isBlockedTag(tag) {
			t.Errorf("SuggestTags returned blocked tag %q: %v", tag, tags)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"auth", "auth"},
		{"  Auth  ", "auth"},
		{"BACKEND", "backend"},
		{"some thing", "some-thing"},
		{"foo--bar", "foo-bar"},
		{"", ""},
		{"   ", ""},
		{"-leading", "leading"},
		{"trailing-", "trailing"},
		// single-char tags are dropped
		{"a", ""},
		// alias resolution
		{"authentication", "auth"},
		{"AUTHORIZATION", "auth"},
		{"database", "db"},
		{"testing", "test"},
		{"deployment", "deploy"},
		{"configuration", "config"},
	}
	for _, tc := range cases {
		got := normalizeTag(tc.input)
		if got != tc.want {
			t.Errorf("normalizeTag(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestObservationTagLimit(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-tag-limit", "mnemo")

	many := make([]string, 20)
	for i := range many {
		many[i] = fmt.Sprintf("tag%02d", i)
	}
	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-tag-limit",
		Type:      "decision",
		Title:     "tag limit test",
		Content:   "test",
		Project:   "mnemo",
		Tags:      many,
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}
	if len(obs.Tags) > maxTagsPerObservation {
		t.Errorf("stored %d tags, want at most %d", len(obs.Tags), maxTagsPerObservation)
	}
}

func TestSessionTagLimit(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-stag-limit", "mnemo")

	many := make([]string, 15)
	for i := range many {
		many[i] = fmt.Sprintf("stag%02d", i)
	}
	if err := s.SetSessionTags("sess-stag-limit", many); err != nil {
		t.Fatalf("SetSessionTags: %v", err)
	}
	sess, err := s.GetSession("sess-stag-limit")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(sess.Tags) > maxTagsPerSession {
		t.Errorf("stored %d session tags, want at most %d", len(sess.Tags), maxTagsPerSession)
	}
}

func TestBlockedTagsNotStored(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-blocked", "mnemo")

	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-blocked",
		Type:      "decision",
		Title:     "blocked tag test",
		Content:   "test",
		Project:   "mnemo",
		Tags:      []string{"auth", "topic", "general", "backend"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}
	tagSet := make(map[string]bool)
	for _, tag := range obs.Tags {
		tagSet[tag] = true
	}
	if tagSet["topic"] {
		t.Error("blocked tag 'topic' was stored")
	}
	if tagSet["general"] {
		t.Error("blocked tag 'general' was stored")
	}
	if !tagSet["auth"] {
		t.Error("valid tag 'auth' was not stored")
	}
	if !tagSet["backend"] {
		t.Error("valid tag 'backend' was not stored")
	}
}

func TestMergeTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-merge", "mnemo")

	id1, _ := s.AddObservation(AddObservationParams{
		SessionID: "sess-merge",
		Type:      "decision",
		Title:     "obs with authentication tag",
		Content:   "test",
		Project:   "mnemo",
		Tags:      []string{"backend", "auth"},
	})
	id2, _ := s.AddObservation(AddObservationParams{
		SessionID: "sess-merge",
		Type:      "decision",
		Title:     "obs with backend tag only",
		Content:   "test",
		Project:   "mnemo",
		Tags:      []string{"backend"},
	})

	obsCount, sessCount, err := s.MergeTags("backend", "infra")
	if err != nil {
		t.Fatalf("MergeTags: %v", err)
	}
	if obsCount != 2 {
		t.Errorf("MergeTags obsCount = %d, want 2", obsCount)
	}
	if sessCount != 0 {
		t.Errorf("MergeTags sessCount = %d, want 0", sessCount)
	}

	obs1, _ := s.GetObservation(id1)
	obs2, _ := s.GetObservation(id2)

	for _, obs := range []*Observation{obs1, obs2} {
		tagSet := make(map[string]bool)
		for _, tag := range obs.Tags {
			tagSet[tag] = true
		}
		if tagSet["backend"] {
			t.Errorf("obs %d still has old tag 'backend' after merge", obs.ID)
		}
		if !tagSet["infra"] {
			t.Errorf("obs %d missing new tag 'infra' after merge", obs.ID)
		}
	}
}

func TestMergeTagsBlockedTarget(t *testing.T) {
	s := newTestStore(t)
	_, _, err := s.MergeTags("auth", "topic")
	if err == nil {
		t.Error("expected error merging into blocked tag 'topic', got nil")
	}
}

// TestMergeTagsLegacyAlias verifies the primary use case: migrating legacy rows
// that predate alias resolution. "database" and "db" would normally be
// "identical after normalization" if both were aliased, but MergeTags preserves
// the raw source so the merge can find and rewrite stored "database" rows.
func TestMergeTagsLegacyAlias(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-legacy", "mnemo")

	// Insert directly with raw tag to simulate pre-alias data.
	id, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-legacy",
		Type:      "decision",
		Title:     "legacy db obs",
		Content:   "test",
		Project:   "mnemo",
		Tags:      []string{"backend"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	// Seed a legacy tag directly in the DB to bypass normalizeTag alias.
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO observation_tags (observation_id, tag) VALUES (?, ?)`, id, "database"); err != nil {
		t.Fatalf("seed legacy tag: %v", err)
	}

	obsCount, _, err := s.MergeTags("database", "db")
	if err != nil {
		t.Fatalf("MergeTags: %v", err)
	}
	if obsCount != 1 {
		t.Errorf("MergeTags obsCount = %d, want 1", obsCount)
	}

	obs, _ := s.GetObservation(id)
	tagSet := make(map[string]bool)
	for _, tag := range obs.Tags {
		tagSet[tag] = true
	}
	if tagSet["database"] {
		t.Error("legacy tag 'database' still present after merge")
	}
	if !tagSet["db"] {
		t.Error("canonical tag 'db' missing after merge")
	}
}

func TestTopicKeyUpsertPreservesTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-topic-tags", "mnemo")

	// First save with topic key and tags
	_, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-topic-tags",
		Type:      "architecture",
		Title:     "auth model",
		Content:   "initial auth design details",
		Project:   "mnemo",
		TopicKey:  "architecture/auth-model",
		Tags:      []string{"auth", "architecture"},
	})
	if err != nil {
		t.Fatalf("AddObservation first: %v", err)
	}

	// Upsert via topic key with different tags
	id2, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-topic-tags",
		Type:      "architecture",
		Title:     "auth model updated",
		Content:   "revised auth design with jwt token",
		Project:   "mnemo",
		TopicKey:  "architecture/auth-model",
		Tags:      []string{"auth", "jwt"},
	})
	if err != nil {
		t.Fatalf("AddObservation upsert: %v", err)
	}

	obs, err := s.GetObservation(id2)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	tagSet := make(map[string]bool)
	for _, tag := range obs.Tags {
		tagSet[tag] = true
	}
	if !tagSet["auth"] || !tagSet["jwt"] {
		t.Errorf("expected tags [auth, jwt] after upsert, got: %v", obs.Tags)
	}
}

func TestExportImportRoundTripPreservesTags(t *testing.T) {
	src := newTestStore(t)
	newTestSession(t, src, "sess-export-tags", "mnemo")

	id1, err := src.AddObservation(AddObservationParams{
		SessionID: "sess-export-tags",
		Type:      "decision",
		Title:     "exported with tags",
		Content:   "this observation has tags that must survive export and import",
		Project:   "mnemo",
		Tags:      []string{"database", "auth", "backend"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	_, err = src.AddObservation(AddObservationParams{
		SessionID: "sess-export-tags",
		Type:      "decision",
		Title:     "exported without tags",
		Content:   "this observation has no tags and should import cleanly",
		Project:   "mnemo",
	})
	if err != nil {
		t.Fatalf("AddObservation (no tags): %v", err)
	}

	// Verify tags exist before export
	orig, err := src.GetObservation(id1)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}
	if len(orig.Tags) != 3 {
		t.Fatalf("pre-export: expected 3 tags, got %d: %v", len(orig.Tags), orig.Tags)
	}

	data, err := src.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Verify Export populated Tags in the export data
	var found bool
	for _, o := range data.Observations {
		if o.Title == "exported with tags" {
			found = true
			if len(o.Tags) != 3 {
				t.Fatalf("export data: expected 3 tags on %q, got %d: %v", o.Title, len(o.Tags), o.Tags)
			}
		}
	}
	if !found {
		t.Fatal("export data did not contain the expected observation")
	}

	// Import into a clean store
	dst := newTestStore(t)
	result, err := dst.Import(data)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.ObservationsImported != 2 {
		t.Fatalf("expected 2 imported observations, got %d", result.ObservationsImported)
	}

	// Verify tags survived in the destination store
	imported, err := dst.AllObservations("mnemo", "project", 10)
	if err != nil {
		t.Fatalf("AllObservations: %v", err)
	}
	if len(imported) != 2 {
		t.Fatalf("expected 2 observations after import, got %d", len(imported))
	}

	tagMap := make(map[string][]string)
	for _, o := range imported {
		tagMap[o.Title] = o.Tags
	}

	// "database" is an alias for "db"; canonical form is stored.
	wantTags := []string{"auth", "backend", "db"}
	gotTags := tagMap["exported with tags"]
	if len(gotTags) != len(wantTags) {
		t.Fatalf("after import: expected tags %v, got %v", wantTags, gotTags)
	}
	tagSet := make(map[string]bool)
	for _, tag := range gotTags {
		tagSet[tag] = true
	}
	for _, want := range wantTags {
		if !tagSet[want] {
			t.Errorf("after import: missing tag %q, got: %v", want, gotTags)
		}
	}

	if len(tagMap["exported without tags"]) != 0 {
		t.Errorf("observation without tags should have none after import, got: %v", tagMap["exported without tags"])
	}
}

// ─── Sync + Tags ─────────────────────────────────────────────────────────────


// ─── Sync + Tags ─────────────────────────────────────────────────────────────

// enrollAndList enrolls project and returns pending mutations.
func enrollAndList(t *testing.T, s *Store, project string) []SyncMutation {
	t.Helper()
	if err := s.EnrollProject(project); err != nil {
		t.Fatalf("EnrollProject: %v", err)
	}
	mutations, err := s.ListPendingSyncMutations(DefaultSyncTargetKey, 50)
	if err != nil {
		t.Fatalf("ListPendingSyncMutations: %v", err)
	}
	return mutations
}

func findObsMutation(t *testing.T, mutations []SyncMutation, syncID string) SyncMutation {
	t.Helper()
	for _, m := range mutations {
		if m.Entity != SyncEntityObservation {
			continue
		}
		if syncID == "" {
			return m
		}
		var p syncObservationPayload
		if err := decodeSyncPayload([]byte(m.Payload), &p); err == nil && p.SyncID == syncID {
			return m
		}
	}
	t.Fatalf("observation mutation not found (syncID=%q)", syncID)
	return SyncMutation{}
}

func TestSyncPayloadIncludesTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-sync-tags", "mnemo")

	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-sync-tags",
		Type:      "decision",
		Title:     "sync payload tags test",
		Content:   "verifies that tags are serialized into the sync mutation payload",
		Project:   "mnemo",
		Tags:      []string{"auth", "backend"},
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	m := findObsMutation(t, enrollAndList(t, s, "mnemo"), "")

	var payload syncObservationPayload
	if err := decodeSyncPayload([]byte(m.Payload), &payload); err != nil {
		t.Fatalf("decodeSyncPayload: %v", err)
	}
	if payload.Tags == nil {
		t.Fatal("payload.Tags is nil — tags were not included in sync payload")
	}
	tagSet := make(map[string]bool)
	for _, tag := range *payload.Tags {
		tagSet[tag] = true
	}
	if !tagSet["auth"] || !tagSet["backend"] {
		t.Errorf("expected tags [auth, backend] in payload, got: %v", *payload.Tags)
	}
}

func TestSyncPayloadEmptyTagsForUntaggedObservation(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-sync-notags", "mnemo")

	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-sync-notags",
		Type:      "decision",
		Title:     "no tags observation",
		Content:   "this observation has no tags at all anywhere",
		Project:   "mnemo",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	m := findObsMutation(t, enrollAndList(t, s, "mnemo"), "")

	var payload syncObservationPayload
	if err := decodeSyncPayload([]byte(m.Payload), &payload); err != nil {
		t.Fatalf("decodeSyncPayload: %v", err)
	}
	if payload.Tags == nil {
		t.Fatal("payload.Tags should be a non-nil empty slice, not nil")
	}
	if len(*payload.Tags) != 0 {
		t.Errorf("expected empty tags, got: %v", *payload.Tags)
	}
}

func TestApplyPulledMutationWithTagsPersistsTags(t *testing.T) {
	src := newTestStore(t)
	newTestSession(t, src, "sess-apply-tags", "mnemo")

	if _, err := src.AddObservation(AddObservationParams{
		SessionID: "sess-apply-tags",
		Type:      "decision",
		Title:     "observation to sync",
		Content:   "content that will travel through sync with tags attached to it",
		Project:   "mnemo",
		Tags:      []string{"sync", "test"},
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	mutations := enrollAndList(t, src, "mnemo")

	dst := newTestStore(t)
	for _, m := range mutations {
		if err := dst.ApplyPulledMutation(DefaultSyncTargetKey, m); err != nil {
			t.Fatalf("ApplyPulledMutation: %v", err)
		}
	}

	obs, err := dst.AllObservations("mnemo", "project", 10)
	if err != nil {
		t.Fatalf("AllObservations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("no observations in destination store after apply")
	}
	tagSet := make(map[string]bool)
	for _, tag := range obs[0].Tags {
		tagSet[tag] = true
	}
	if !tagSet["sync"] || !tagSet["test"] {
		t.Errorf("expected tags [sync, test] after apply, got: %v", obs[0].Tags)
	}
}

func TestApplyPulledMutationClearsTagsOnUpdate(t *testing.T) {
	src := newTestStore(t)
	newTestSession(t, src, "sess-clear-sync", "mnemo")

	id, err := src.AddObservation(AddObservationParams{
		SessionID: "sess-clear-sync",
		Type:      "decision",
		Title:     "will lose tags via sync update",
		Content:   "initial content with tags that will be removed on next update",
		Project:   "mnemo",
		Tags:      []string{"old-tag"},
	})
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	origObs, err := src.GetObservation(id)
	if err != nil {
		t.Fatalf("GetObservation: %v", err)
	}

	emptyTags := []string{}
	if _, err := src.UpdateObservation(id, UpdateObservationParams{Tags: &emptyTags}); err != nil {
		t.Fatalf("UpdateObservation: %v", err)
	}

	mutations := enrollAndList(t, src, "mnemo")

	// Find the last mutation for this observation (the update)
	updateMutation := findObsMutation(t, mutations[len(mutations)-1:], origObs.SyncID)

	dst := newTestStore(t)
	// Apply all mutations in order (session + insert + update)
	for _, m := range mutations {
		if err := dst.ApplyPulledMutation(DefaultSyncTargetKey, m); err != nil {
			t.Fatalf("ApplyPulledMutation (seq=%d): %v", m.Seq, err)
		}
	}
	_ = updateMutation

	dstObs, err := dst.AllObservations("mnemo", "project", 10)
	if err != nil {
		t.Fatalf("AllObservations: %v", err)
	}
	if len(dstObs) == 0 {
		t.Fatal("no observations after apply")
	}
	if len(dstObs[0].Tags) != 0 {
		t.Errorf("expected no tags after clear via sync, got: %v", dstObs[0].Tags)
	}
}

func TestSetSessionTagsPersistsAndLoads(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-tag-session", "mnemo")

	if err := s.SetSessionTags("sess-tag-session", []string{"backend", "AUTH", "back-end"}); err != nil {
		t.Fatalf("SetSessionTags: %v", err)
	}

	sess, err := s.GetSession("sess-tag-session")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	tagSet := make(map[string]bool)
	for _, tag := range sess.Tags {
		tagSet[tag] = true
	}
	if !tagSet["backend"] {
		t.Errorf("expected tag 'backend', got: %v", sess.Tags)
	}
	if !tagSet["auth"] {
		t.Errorf("expected tag 'auth' (normalized from 'AUTH'), got: %v", sess.Tags)
	}
	// "back-end" normalizes to "back-end" but "backend" is already in the set —
	// both should be present since they normalize differently.
	if !tagSet["back-end"] {
		t.Errorf("expected tag 'back-end', got: %v", sess.Tags)
	}
}

func TestSessionTagsExportImportRoundTrip(t *testing.T) {
	src := newTestStore(t)
	newTestSession(t, src, "sess-export-tags", "mnemo")

	if err := src.SetSessionTags("sess-export-tags", []string{"feature", "sync"}); err != nil {
		t.Fatalf("SetSessionTags: %v", err)
	}

	data, err := src.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	var exported *Session
	for i := range data.Sessions {
		if data.Sessions[i].ID == "sess-export-tags" {
			exported = &data.Sessions[i]
			break
		}
	}
	if exported == nil {
		t.Fatal("session not found in export")
	}
	tagSet := make(map[string]bool)
	for _, tag := range exported.Tags {
		tagSet[tag] = true
	}
	if !tagSet["feature"] || !tagSet["sync"] {
		t.Errorf("expected tags [feature, sync] in export, got: %v", exported.Tags)
	}

	dst := newTestStore(t)
	if _, err := dst.Import(data); err != nil {
		t.Fatalf("Import: %v", err)
	}

	sess, err := dst.GetSession("sess-export-tags")
	if err != nil {
		t.Fatalf("GetSession after import: %v", err)
	}
	tagSet2 := make(map[string]bool)
	for _, tag := range sess.Tags {
		tagSet2[tag] = true
	}
	if !tagSet2["feature"] || !tagSet2["sync"] {
		t.Errorf("expected tags [feature, sync] after import, got: %v", sess.Tags)
	}
}

func TestSessionTagsSyncPayloadIncludesTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-sync-session-tags", "mnemo")

	if err := s.SetSessionTags("sess-sync-session-tags", []string{"api", "refactor"}); err != nil {
		t.Fatalf("SetSessionTags: %v", err)
	}

	mutations := enrollAndList(t, s, "mnemo")

	var sessionMutation *SyncMutation
	for i := range mutations {
		if mutations[i].Entity == SyncEntitySession && mutations[i].EntityKey == "sess-sync-session-tags" {
			sessionMutation = &mutations[i]
		}
	}
	if sessionMutation == nil {
		t.Fatal("no session mutation found")
	}

	var payload syncSessionPayload
	if err := decodeSyncPayload([]byte(sessionMutation.Payload), &payload); err != nil {
		t.Fatalf("decodeSyncPayload: %v", err)
	}
	if payload.Tags == nil {
		t.Fatal("payload.Tags is nil — session tags not included in sync payload")
	}
	tagSet := make(map[string]bool)
	for _, tag := range *payload.Tags {
		tagSet[tag] = true
	}
	if !tagSet["api"] || !tagSet["refactor"] {
		t.Errorf("expected tags [api, refactor] in session payload, got: %v", *payload.Tags)
	}
}

func TestSuggestTagsNoSubstringMatch(t *testing.T) {
	// "decision" as type should not suggest "ci" (substring of "decision")
	tags := SuggestTags("decision", "some decision about architecture", "we decided to use X")
	for _, tag := range tags {
		if tag == "ci" {
			t.Errorf("SuggestTags incorrectly suggested 'ci' from 'decision' via substring match")
		}
	}
}

func TestSuggestTagsExactTokenMatch(t *testing.T) {
	tags := SuggestTags("bugfix", "ci pipeline fix", "the ci job was failing due to missing env")
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}
	if !tagSet["ci"] {
		t.Errorf("expected 'ci' tag when 'ci' appears as a whole word, got: %v", tags)
	}
}

// ─── Issue #5: query-optional Search ─────────────────────────────────────────

func TestSearchEmptyQueryByTag(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-browse", "mnemo")

	addObs := func(title, content string, tags []string) {
		t.Helper()
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-browse",
			Type:      "decision",
			Title:     title,
			Content:   content,
			Project:   "mnemo",
			Tags:      tags,
		}); err != nil {
			t.Fatalf("AddObservation(%q): %v", title, err)
		}
	}

	// Inserted in order: oldest first. id DESC tiebreaker means last inserted appears first.
	addObs("auth middleware", "JWT implementation", []string{"auth", "backend"})
	addObs("cache layer", "Redis setup", []string{"cache", "backend"})
	addObs("database schema", "migrations", []string{"database"})

	results, err := s.Search("", SearchOptions{Project: "mnemo", Tags: []string{"backend"}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for tag 'backend', got %d", len(results))
	}
	for _, r := range results {
		found := false
		for _, tag := range r.Tags {
			if tag == "backend" {
				found = true
			}
		}
		if !found {
			t.Errorf("result %q missing expected tag 'backend'", r.Title)
		}
	}
	// Verify ordering: most recently inserted (highest id) first.
	if results[0].Title != "cache layer" {
		t.Errorf("expected 'cache layer' first (most recent), got: %q", results[0].Title)
	}
	if results[1].Title != "auth middleware" {
		t.Errorf("expected 'auth middleware' second, got: %q", results[1].Title)
	}
}

func TestSearchEmptyQueryByTopicKey(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-topickey", "mnemo")

	// Note: AddObservation upserts by topic_key — each unique key gets one observation.
	for i, tk := range []string{"auth/jwt", "auth/session", "cache/redis"} {
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-topickey",
			Type:      "decision",
			Title:     fmt.Sprintf("obs %d", i),
			Content:   fmt.Sprintf("unique content for topic %s", tk),
			Project:   "mnemo",
			TopicKey:  tk,
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	// Filter by exact topic_key — should return exactly 1
	results, err := s.Search("", SearchOptions{Project: "mnemo", TopicKey: "auth/jwt"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for topic_key 'auth/jwt', got %d", len(results))
	}
	if derefString(results[0].TopicKey) != "auth/jwt" {
		t.Errorf("result has wrong topic_key: %v", results[0].TopicKey)
	}

	// Non-matching topic_key — should return 0
	results2, err := s.Search("", SearchOptions{Project: "mnemo", TopicKey: "auth"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results2) != 0 {
		t.Fatalf("expected 0 results for non-matching topic_key 'auth', got %d", len(results2))
	}
}

// ─── Issue #6 + #7: PreferTags y TopicKey boost ───────────────────────────────

func TestSearchPreferTagsRankFirst(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-boost", "mnemo")

	// Tagged obs is inserted first (lower id = older). Without boost it would appear
	// last (id DESC tiebreaker). With boost it must appear first.
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-boost",
		Type:      "decision",
		Title:     "auth decision",
		Content:   "some architecture note about auth and security concerns",
		Project:   "mnemo",
		Tags:      []string{"auth"},
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-boost",
		Type:      "decision",
		Title:     "general decision",
		Content:   "some architecture note about the system design",
		Project:   "mnemo",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	// Baseline: without boost, untagged (newer) should appear first.
	baseline, err := s.Search("architecture note", SearchOptions{Project: "mnemo"})
	if err != nil {
		t.Fatalf("Search (baseline): %v", err)
	}
	if len(baseline) < 2 {
		t.Fatalf("baseline: expected at least 2 results, got %d", len(baseline))
	}
	if baseline[0].Title != "general decision" {
		t.Errorf("baseline: expected 'general decision' first (newer), got: %q", baseline[0].Title)
	}

	// With boost: tagged (older) must appear first.
	results, err := s.Search("architecture note", SearchOptions{
		Project:   "mnemo",
		PreferTags: []string{"auth"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Title != "auth decision" {
		t.Errorf("expected auth-tagged observation first with boost, got: %q", results[0].Title)
	}
}

func TestRecentObservationsPreferTagsRankFirst(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ctx-boost", "mnemo")

	// Tagged obs inserted first (lower id = older). Without boost it appears last.
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-ctx-boost",
		Type:      "decision",
		Title:     "tagged older",
		Content:   "has auth tag",
		Project:   "mnemo",
		Tags:      []string{"auth"},
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-ctx-boost",
		Type:      "decision",
		Title:     "untagged newer",
		Content:   "no tags",
		Project:   "mnemo",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	// Baseline: without boost, untagged (newer) appears first.
	baseline, err := s.RecentObservationsOpts("mnemo", "project", 10, ContextOptions{})
	if err != nil {
		t.Fatalf("RecentObservationsOpts (baseline): %v", err)
	}
	if len(baseline) < 2 {
		t.Fatalf("baseline: expected 2 observations, got %d", len(baseline))
	}
	if baseline[0].Title != "untagged newer" {
		t.Errorf("baseline: expected 'untagged newer' first, got: %q", baseline[0].Title)
	}

	// With boost: tagged (older) must appear first.
	obs, err := s.RecentObservationsOpts("mnemo", "project", 10, ContextOptions{
		PreferTags: []string{"auth"},
	})
	if err != nil {
		t.Fatalf("RecentObservationsOpts: %v", err)
	}
	if len(obs) < 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	if obs[0].Title != "tagged older" {
		t.Errorf("expected auth-tagged observation first with boost, got: %q", obs[0].Title)
	}
}

func TestRecentObservationsTopicKeyBoost(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-tk-boost", "mnemo")

	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-tk-boost",
		Type:      "decision",
		Title:     "unrelated",
		Content:   "no topic key",
		Project:   "mnemo",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-tk-boost",
		Type:      "decision",
		Title:     "auth jwt observation",
		Content:   "has topic key",
		Project:   "mnemo",
		TopicKey:  "auth/jwt",
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	obs, err := s.RecentObservationsOpts("mnemo", "project", 10, ContextOptions{
		TopicKey: "auth/jwt",
	})
	if err != nil {
		t.Fatalf("RecentObservationsOpts: %v", err)
	}
	if len(obs) < 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	if obs[0].Title != "auth jwt observation" {
		t.Errorf("expected topic_key observation first, got: %q", obs[0].Title)
	}
	if obs[1].Title != "unrelated" {
		t.Errorf("expected unrelated observation second, got: %q", obs[1].Title)
	}
}

// ─── Issue #8: ListTags ───────────────────────────────────────────────────────

func TestListTagsReturnsFrequency(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-listtags", "mnemo")

	add := func(title string, tags []string) {
		t.Helper()
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-listtags",
			Type:      "decision",
			Title:     title,
			Content:   "content",
			Project:   "mnemo",
			Tags:      tags,
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	add("obs 1", []string{"auth", "backend"})
	// "database" is an alias for "db"; the stored canonical tag is "db".
	add("obs 2", []string{"auth", "database"})
	add("obs 3", []string{"auth"})

	tags, err := s.ListTags("mnemo")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	byTag := make(map[string]TagInfo)
	for _, ti := range tags {
		byTag[ti.Tag] = ti
	}

	if byTag["auth"].Count != 3 {
		t.Errorf("expected auth count=3, got %d", byTag["auth"].Count)
	}
	if byTag["backend"].Count != 1 {
		t.Errorf("expected backend count=1, got %d", byTag["backend"].Count)
	}
	if byTag["db"].Count != 1 {
		t.Errorf("expected db count=1, got %d", byTag["db"].Count)
	}
	if tags[0].Tag != "auth" {
		t.Errorf("expected auth first by frequency, got %q", tags[0].Tag)
	}
	if byTag["auth"].LastUsedAt == "" {
		t.Error("expected non-empty last_used_at for auth")
	}
}

func TestListTagsFiltersByProject(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-lt-proj", "proj-a")
	newTestSession(t, s, "sess-lt-other", "proj-b")

	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-lt-proj",
		Type:      "decision",
		Title:     "proj-a obs",
		Content:   "content",
		Project:   "proj-a",
		Tags:      []string{"exclusive-tag"},
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "sess-lt-other",
		Type:      "decision",
		Title:     "proj-b obs",
		Content:   "content",
		Project:   "proj-b",
		Tags:      []string{"other-tag"},
	}); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	tags, err := s.ListTags("proj-a")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	for _, ti := range tags {
		if ti.Tag == "other-tag" {
			t.Errorf("ListTags(proj-a) returned tag from proj-b: %q", ti.Tag)
		}
	}
	found := false
	for _, ti := range tags {
		if ti.Tag == "exclusive-tag" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'exclusive-tag' in ListTags(proj-a)")
	}
}

// ─── TagStats ─────────────────────────────────────────────────────────────────

func TestTagStatsMinCount(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-min", "proj-min")

	add := func(title string, tags []string) {
		t.Helper()
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-min",
			Type:      "decision",
			Title:     title,
			Content:   "content",
			Project:   "proj-min",
			Tags:      tags,
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	// "common" appears 4 times, "rare" appears once.
	add("obs1", []string{"common"})
	add("obs2", []string{"common"})
	add("obs3", []string{"common"})
	add("obs4", []string{"common", "rare"})

	// min_count=3 must return only "common".
	tags, err := s.TagStats("proj-min", TagStatsOptions{MinCount: 3})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag with MinCount=3, got %d: %v", len(tags), tags)
	}
	if tags[0].Tag != "common" {
		t.Errorf("expected 'common', got %q", tags[0].Tag)
	}
	if tags[0].Count != 4 {
		t.Errorf("expected count=4, got %d", tags[0].Count)
	}
}

func TestTagStatsMaxCount(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-max", "proj-max")

	add := func(title string, tags []string) {
		t.Helper()
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-max",
			Type:      "decision",
			Title:     title,
			Content:   "content",
			Project:   "proj-max",
			Tags:      tags,
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	// "noisy" appears 5 times, "low" appears once.
	add("obs1", []string{"noisy"})
	add("obs2", []string{"noisy"})
	add("obs3", []string{"noisy"})
	add("obs4", []string{"noisy"})
	add("obs5", []string{"noisy", "low"})

	// max_count=2 must return only "low".
	tags, err := s.TagStats("proj-max", TagStatsOptions{MaxCount: 2})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag with MaxCount=2, got %d: %v", len(tags), tags)
	}
	if tags[0].Tag != "low" {
		t.Errorf("expected 'low', got %q", tags[0].Tag)
	}
}

func TestTagStatsUnusedSince(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-stale", "proj-stale")

	add := func(title string, tags []string) int64 {
		t.Helper()
		id, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-stale",
			Type:      "decision",
			Title:     title,
			Content:   "content",
			Project:   "proj-stale",
			Tags:      tags,
		})
		if err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
		return id
	}

	freshID := add("obs1", []string{"fresh"})
	staleID := add("obs2", []string{"stale-tag"})

	// Pin both timestamps so the test does not depend on the current clock.
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fresh := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	if _, err := s.db.Exec(
		"UPDATE observations SET created_at = ? WHERE id = ?",
		fresh.UTC().Format(time.RFC3339),
		freshID,
	); err != nil {
		t.Fatalf("set fresh observation time: %v", err)
	}
	if _, err := s.db.Exec(
		"UPDATE observations SET created_at = ? WHERE id = ?",
		past.UTC().Format(time.RFC3339),
		staleID,
	); err != nil {
		t.Fatalf("backdate observation: %v", err)
	}

	tags, err := s.TagStats("proj-stale", TagStatsOptions{UnusedSince: cutoff})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 stale tag, got %d: %v", len(tags), tags)
	}
	if tags[0].Tag != "stale-tag" {
		t.Errorf("expected 'stale-tag', got %q", tags[0].Tag)
	}
}

func TestTagStatsSortByStale(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-sort", "proj-sort")

	// tag names are lowercase after normalization.
	type tagEntry struct {
		name string
		ts   time.Time
	}
	entries := []tagEntry{
		{"tag-alpha", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}, // newest
		{"tag-beta", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},  // middle
		{"tag-gamma", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, // oldest
	}
	for i, e := range entries {
		id, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-sort",
			Type:      "decision",
			Title:     fmt.Sprintf("obs%d", i),
			Content:   "content",
			Project:   "proj-sort",
			Tags:      []string{e.name},
		})
		if err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
		if _, err := s.db.Exec(
			"UPDATE observations SET created_at = ? WHERE id = ?",
			e.ts.UTC().Format(time.RFC3339), id,
		); err != nil {
			t.Fatalf("backdate observation id=%d: %v", id, err)
		}
	}

	tags, err := s.TagStats("proj-sort", TagStatsOptions{SortBy: "stale"})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	// stale sort: oldest first → tag-gamma, tag-beta, tag-alpha.
	if tags[0].Tag != "tag-gamma" {
		t.Errorf("expected tag-gamma first (oldest), got %q", tags[0].Tag)
	}
	if tags[2].Tag != "tag-alpha" {
		t.Errorf("expected tag-alpha last (newest), got %q", tags[2].Tag)
	}
}

func TestTagStatsLimit(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-limit", "proj-limit")

	for i := 0; i < 5; i++ {
		tag := fmt.Sprintf("tag%d", i)
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-limit",
			Type:      "decision",
			Title:     tag,
			Content:   "content",
			Project:   "proj-limit",
			Tags:      []string{tag},
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	tags, err := s.TagStats("proj-limit", TagStatsOptions{Limit: 3})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 results with Limit=3, got %d", len(tags))
	}
}

func TestTagStatsDoesNotBreakListTags(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-compat", "proj-compat")

	add := func(title string, tags []string) {
		t.Helper()
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-compat",
			Type:      "decision",
			Title:     title,
			Content:   "content",
			Project:   "proj-compat",
			Tags:      tags,
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	add("obs1", []string{"auth", "backend"})
	add("obs2", []string{"auth"})

	listResult, err := s.ListTags("proj-compat")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	statsResult, err := s.TagStats("proj-compat", TagStatsOptions{})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}

	// Both must return the same tags (order may differ on tie, but same set and counts).
	if len(listResult) != len(statsResult) {
		t.Errorf("ListTags returned %d tags, TagStats returned %d", len(listResult), len(statsResult))
	}
	byTag := make(map[string]int, len(listResult))
	for _, ti := range listResult {
		byTag[ti.Tag] = ti.Count
	}
	for _, ti := range statsResult {
		if byTag[ti.Tag] != ti.Count {
			t.Errorf("tag %q: ListTags count=%d, TagStats count=%d", ti.Tag, byTag[ti.Tag], ti.Count)
		}
	}
}

func TestTagWeightsBasic(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	tags := []TagInfo{
		{Tag: "auth", Count: 10, LastUsedAt: "2026-04-05T00:00:00Z"},
		{Tag: "backend", Count: 5, LastUsedAt: "2026-03-01T00:00:00Z"},
		{Tag: "legacy", Count: 1, LastUsedAt: "2026-01-01T00:00:00Z"},
	}

	weights := tagWeights(tags, 2, now)
	if len(weights) != 3 {
		t.Fatalf("expected 3 weights, got %d", len(weights))
	}

	byTag := make(map[string]TagWeight, len(weights))
	for _, w := range weights {
		byTag[w.Tag] = w
	}

	// Top 2 by frequency: auth (10) and backend (5) are dominant.
	if !byTag["auth"].IsDominant {
		t.Error("auth should be dominant (top 2)")
	}
	if !byTag["backend"].IsDominant {
		t.Error("backend should be dominant (top 2)")
	}
	if byTag["legacy"].IsDominant {
		t.Error("legacy should not be dominant (rank 3)")
	}

	// auth is the newest → RecencyScore should be 1.0.
	if byTag["auth"].RecencyScore != 1.0 {
		t.Errorf("auth RecencyScore: expected 1.0, got %f", byTag["auth"].RecencyScore)
	}
	// legacy is the oldest → RecencyScore should be 0.0.
	if byTag["legacy"].RecencyScore != 0.0 {
		t.Errorf("legacy RecencyScore: expected 0.0, got %f", byTag["legacy"].RecencyScore)
	}
	// backend is in the middle → 0 < score < 1.
	if byTag["backend"].RecencyScore <= 0 || byTag["backend"].RecencyScore >= 1 {
		t.Errorf("backend RecencyScore should be between 0 and 1, got %f", byTag["backend"].RecencyScore)
	}
}

func TestTagStatsSortByAlpha(t *testing.T) {
	s := newTestStore(t)
	newTestSession(t, s, "sess-ts-alpha", "proj-alpha")

	for _, tag := range []string{"zeta", "alpha", "mango"} {
		if _, err := s.AddObservation(AddObservationParams{
			SessionID: "sess-ts-alpha",
			Type:      "decision",
			Title:     tag,
			Content:   "content",
			Project:   "proj-alpha",
			Tags:      []string{tag},
		}); err != nil {
			t.Fatalf("AddObservation: %v", err)
		}
	}

	tags, err := s.TagStats("proj-alpha", TagStatsOptions{SortBy: "alpha"})
	if err != nil {
		t.Fatalf("TagStats: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	if tags[0].Tag != "alpha" || tags[1].Tag != "mango" || tags[2].Tag != "zeta" {
		t.Fatalf("unexpected alpha ordering: %v %v %v", tags[0].Tag, tags[1].Tag, tags[2].Tag)
	}
}
