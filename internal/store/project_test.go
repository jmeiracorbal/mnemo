package store

import (
	"strings"
	"testing"
)

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

func TestBuildProjectMergePlan(t *testing.T) {
	s := newTestStore(t)

	if err := s.EnsureProject("alpha-legacy", "Alpha Legacy"); err != nil {
		t.Fatalf("ensure source project: %v", err)
	}
	if err := s.EnsureProject("11111111-2222-3333-4444-555555555555", "Alpha"); err != nil {
		t.Fatalf("ensure destination project: %v", err)
	}
	if err := s.CreateSession("s-alpha-legacy", "alpha-legacy", "/tmp/alpha"); err != nil {
		t.Fatalf("create source session: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "s-alpha-legacy",
		Type:      "decision",
		Title:     "Alpha decision",
		Content:   "Alpha memory",
		Project:   "alpha-legacy",
		Scope:     "project",
	}); err != nil {
		t.Fatalf("add source observation: %v", err)
	}
	if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-alpha-legacy", Content: "Alpha prompt", Project: "alpha-legacy"}); err != nil {
		t.Fatalf("add source prompt: %v", err)
	}
	if err := s.EnrollProject("alpha-legacy"); err != nil {
		t.Fatalf("enroll source project: %v", err)
	}

	plan, err := s.BuildProjectMergePlan("alpha-legacy", "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatalf("build project merge plan: %v", err)
	}
	if plan.From.ID != "alpha-legacy" || plan.To.ID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("unexpected plan endpoints: %+v", plan)
	}
	if plan.Observations != 1 || plan.Sessions != 1 || plan.Prompts != 1 || plan.SyncMutations != 3 {
		t.Fatalf("unexpected plan counts: %+v", plan)
	}
	if !plan.SourceEnrolled || plan.DestinationEnrolled || !plan.WillCopyEnrollment || !plan.WillDeleteSourceProject {
		t.Fatalf("unexpected plan flags: %+v", plan)
	}
}

func TestMergeProjectsConsolidatesProjectData(t *testing.T) {
	s := newTestStore(t)
	source := "alpha-legacy"
	destination := "11111111-2222-3333-4444-555555555555"

	if err := s.EnsureProject(source, "Alpha Legacy"); err != nil {
		t.Fatalf("ensure source project: %v", err)
	}
	if err := s.EnsureProject(destination, "Alpha"); err != nil {
		t.Fatalf("ensure destination project: %v", err)
	}
	if err := s.CreateSession("s-alpha-legacy", source, "/tmp/alpha"); err != nil {
		t.Fatalf("create source session: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "s-alpha-legacy",
		Type:      "decision",
		Title:     "Alpha merge",
		Content:   "Merge source data",
		Project:   source,
		Scope:     "project",
	}); err != nil {
		t.Fatalf("add source observation: %v", err)
	}
	if _, err := s.AddPrompt(AddPromptParams{SessionID: "s-alpha-legacy", Content: "Alpha prompt", Project: source}); err != nil {
		t.Fatalf("add source prompt: %v", err)
	}
	if err := s.EnrollProject(source); err != nil {
		t.Fatalf("enroll source project: %v", err)
	}

	result, err := s.MergeProjects(source, destination)
	if err != nil {
		t.Fatalf("merge projects: %v", err)
	}
	if !result.Merged || result.ObservationsUpdated != 1 || result.SessionsUpdated != 1 || result.PromptsUpdated != 1 || result.SyncMutationsUpdated != 3 {
		t.Fatalf("unexpected merge result: %+v", result)
	}
	if !result.SourceProjectDeleted || !result.EnrollmentTransferred {
		t.Fatalf("expected source metadata deletion and enrollment transfer: %+v", result)
	}

	if project, err := s.GetProjectByID(source); err != nil {
		t.Fatalf("get source project: %v", err)
	} else if project != nil {
		t.Fatalf("source project metadata still exists: %+v", project)
	}
	sourceEnrolled, err := s.IsProjectEnrolled(source)
	if err != nil {
		t.Fatalf("check source enrollment: %v", err)
	}
	destinationEnrolled, err := s.IsProjectEnrolled(destination)
	if err != nil {
		t.Fatalf("check destination enrollment: %v", err)
	}
	if sourceEnrolled || !destinationEnrolled {
		t.Fatalf("unexpected enrollment state: source=%v destination=%v", sourceEnrolled, destinationEnrolled)
	}

	projects, err := s.ListProjectSummaries()
	if err != nil {
		t.Fatalf("list project summaries: %v", err)
	}
	if findProjectSummary(projects, source) != nil {
		t.Fatalf("source project still appears in summaries: %+v", projects)
	}
	summary := findProjectSummary(projects, destination)
	if summary == nil {
		t.Fatalf("destination project missing from summaries: %+v", projects)
	}
	if summary.ObservationCount != 1 || summary.SessionCount != 1 || summary.PromptCount != 1 {
		t.Fatalf("unexpected destination summary: %+v", summary)
	}

	results, err := s.Search("Alpha merge", SearchOptions{Project: destination, Limit: 10})
	if err != nil {
		t.Fatalf("search destination project: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected destination search result, got %d", len(results))
	}
	results, err = s.Search("Alpha merge", SearchOptions{Project: source, Limit: 10})
	if err != nil {
		t.Fatalf("search source project: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no source search results, got %d", len(results))
	}

	var sourceMutations, destinationMutations int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE project = ?`, source).Scan(&sourceMutations); err != nil {
		t.Fatalf("count source sync mutations: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE project = ?`, destination).Scan(&destinationMutations); err != nil {
		t.Fatalf("count destination sync mutations: %v", err)
	}
	if sourceMutations != 0 || destinationMutations != 3 {
		t.Fatalf("unexpected sync mutation counts: source=%d destination=%d", sourceMutations, destinationMutations)
	}
}

func TestRenameProjectUpdatesMetadataWithoutChangingActivityProjectID(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-alpha", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("create alpha session: %v", err)
	}
	if _, err := s.AddObservation(AddObservationParams{
		SessionID: "s-alpha",
		Type:      "decision",
		Title:     "Alpha rename",
		Content:   "Keep activity project id stable",
		Project:   "alpha",
		Scope:     "project",
	}); err != nil {
		t.Fatalf("add alpha observation: %v", err)
	}

	result, err := s.RenameProject(ProjectRenameSelector{ID: "alpha"}, "Alpha Project")
	if err != nil {
		t.Fatalf("rename project: %v", err)
	}
	if !result.Renamed || result.Plan.Project.ID != "alpha" || result.Plan.NewName != "Alpha Project" {
		t.Fatalf("unexpected rename result: %+v", result)
	}

	project, err := s.GetProjectByID("alpha")
	if err != nil {
		t.Fatalf("get project metadata: %v", err)
	}
	if project == nil || project.Name != "Alpha Project" {
		t.Fatalf("project metadata = %+v, want renamed metadata", project)
	}

	projects, err := s.ListProjectSummaries()
	if err != nil {
		t.Fatalf("list project summaries: %v", err)
	}
	summary := findProjectSummary(projects, "alpha")
	if summary == nil || summary.Name != "Alpha Project" || summary.ObservationCount != 1 || summary.SessionCount != 1 {
		t.Fatalf("unexpected renamed summary: %+v in %+v", summary, projects)
	}

	results, err := s.Search("Alpha rename", SearchOptions{Project: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("search renamed project id: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected activity to remain under project id alpha, got %d", len(results))
	}
	results, err = s.Search("Alpha rename", SearchOptions{Project: "Alpha Project", Limit: 10})
	if err != nil {
		t.Fatalf("search display name: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no activity under display name, got %d", len(results))
	}
}

func TestRenameProjectSelectsByPath(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-alpha", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("create alpha session: %v", err)
	}

	result, err := s.RenameProject(ProjectRenameSelector{Path: "/tmp/alpha/."}, "Alpha Project")
	if err != nil {
		t.Fatalf("rename project by path: %v", err)
	}
	if !result.Renamed || result.Plan.Selector != "path" || result.Plan.SelectorValue != "/tmp/alpha" {
		t.Fatalf("unexpected path rename result: %+v", result)
	}
	project, err := s.GetProjectByID("alpha")
	if err != nil {
		t.Fatalf("get project metadata: %v", err)
	}
	if project == nil || project.Name != "Alpha Project" {
		t.Fatalf("project metadata = %+v, want renamed metadata", project)
	}
}

func TestBuildProjectRenamePlanByPathRejectsAmbiguousPath(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateSession("s-alpha", "alpha", "/tmp/shared"); err != nil {
		t.Fatalf("create alpha session: %v", err)
	}
	if err := s.CreateSession("s-beta", "beta", "/tmp/shared"); err != nil {
		t.Fatalf("create beta session: %v", err)
	}

	_, err := s.BuildProjectRenamePlan(ProjectRenameSelector{Path: "/tmp/shared/."}, "Shared")
	if err == nil {
		t.Fatal("expected ambiguous path error")
	}
	if !strings.Contains(err.Error(), "matches multiple projects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildProjectMergePlanRejectsMissingProjects(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureProject("alpha", "Alpha"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if _, err := s.BuildProjectMergePlan("missing", "alpha"); err == nil {
		t.Fatal("expected missing source error")
	}
	if _, err := s.BuildProjectMergePlan("alpha", "missing"); err == nil {
		t.Fatal("expected missing destination error")
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
