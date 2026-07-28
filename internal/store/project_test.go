package store

import "testing"

func TestListProjectSummariesIncludesRegisteredAndActivityProjects(t *testing.T) {
	s := newTestStore(t)

	if err := s.EnsureProject("registered", "Registered"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if err := s.CreateSession("s-alpha", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("create alpha session: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "s-alpha",
		Type:      "decision",
		Title:     "Alpha",
		Content:   "Alpha memory",
		Project:   "alpha",
		Scope:     "project",
	}); err != nil {
		t.Fatalf("add alpha observation: %v", err)
	}
	if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-alpha", Content: "Alpha prompt", Project: "alpha"}); err != nil {
		t.Fatalf("add alpha prompt: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE sessions SET started_at = '2026-01-01 10:00:00' WHERE id = 's-alpha'`); err != nil {
		t.Fatalf("update alpha session: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE observations SET created_at = '2026-01-02 10:00:00', updated_at = '2026-01-02 10:00:00', last_seen_at = '2026-01-02 10:00:00' WHERE project = 'alpha'`); err != nil {
		t.Fatalf("update alpha observation: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE user_prompts SET created_at = '2026-01-03 10:00:00' WHERE project = 'alpha'`); err != nil {
		t.Fatalf("update alpha prompt: %v", err)
	}

	projects, err := s.ListProjectSummaries()
	if err != nil {
		t.Fatalf("list project summaries: %v", err)
	}

	registered := findProjectSummary(projects, "registered")
	if registered == nil {
		t.Fatalf("registered project missing from summaries: %+v", projects)
	}
	if registered.Name != "Registered" || registered.ObservationCount != 0 || registered.LastSeenAt != "" {
		t.Fatalf("unexpected registered summary: %+v", registered)
	}

	alpha := findProjectSummary(projects, "alpha")
	if alpha == nil {
		t.Fatalf("activity project missing from summaries: %+v", projects)
	}
	if alpha.Name != "alpha" || alpha.SessionCount != 1 || alpha.ObservationCount != 1 || alpha.PromptCount != 1 {
		t.Fatalf("unexpected alpha counts: %+v", alpha)
	}
	if alpha.Directory != "/tmp/alpha" {
		t.Fatalf("directory = %q, want latest session directory", alpha.Directory)
	}
	if alpha.LastSeenAt != "2026-01-03 10:00:00" {
		t.Fatalf("last_seen_at = %q, want prompt timestamp", alpha.LastSeenAt)
	}
}

func findProjectSummary(projects []ProjectSummary, id string) *ProjectSummary {
	for i := range projects {
		if projects[i].ID == id {
			return &projects[i]
		}
	}
	return nil
}
