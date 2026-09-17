package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

func TestParseProjectsListArgsDefaults(t *testing.T) {
	opts, err := parseProjectsListArgs(nil, fixedProjectsNow)
	if err != nil {
		t.Fatalf("parse projects list args: %v", err)
	}
	if opts.SortBy != "last-seen" || !opts.Desc {
		t.Fatalf("unexpected defaults: %+v", opts)
	}
}

func TestParseProjectsListArgsWithSortAndFilters(t *testing.T) {
	opts, err := parseProjectsListArgs([]string{"--sort=observations", "--asc", "--unused-since=30d", "--empty", "--json"}, fixedProjectsNow)
	if err != nil {
		t.Fatalf("parse projects list args: %v", err)
	}
	if opts.SortBy != "observations" || opts.Desc || !opts.Empty || !opts.JSON {
		t.Fatalf("unexpected options: %+v", opts)
	}
	wantCutoff := fixedProjectsNow().Add(-30 * 24 * time.Hour)
	if !opts.UnusedSince.Equal(wantCutoff) {
		t.Fatalf("cutoff = %s, want %s", opts.UnusedSince, wantCutoff)
	}
}

func TestParseProjectsListArgsRejectsUnknownSort(t *testing.T) {
	_, err := parseProjectsListArgs([]string{"--sort=bogus"}, fixedProjectsNow)
	if err == nil {
		t.Fatal("expected unsupported sort error")
	}
}

func TestParseProjectsMergeArgsExplicitDryRun(t *testing.T) {
	opts, err := parseProjectsMergeArgs([]string{"--from=alpha-legacy", "--to=alpha", "--dry-run", "--json"})
	if err != nil {
		t.Fatalf("parse projects merge args: %v", err)
	}
	if opts.From != "alpha-legacy" || opts.To != "alpha" || !opts.DryRun || !opts.JSON || opts.Yes || opts.AutoByPath {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseProjectsMergeArgsRequiresSafetyFlag(t *testing.T) {
	_, err := parseProjectsMergeArgs([]string{"--from=alpha-legacy", "--to=alpha"})
	if err == nil {
		t.Fatal("expected safety flag error")
	}
}

func TestParseProjectsMergeArgsAutoByPath(t *testing.T) {
	opts, err := parseProjectsMergeArgs([]string{"--auto-by-path", "--dry-run"})
	if err != nil {
		t.Fatalf("parse projects merge args: %v", err)
	}
	if !opts.AutoByPath || !opts.DryRun {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if _, err := parseProjectsMergeArgs([]string{"--auto-by-path", "--from=alpha", "--dry-run"}); err == nil {
		t.Fatal("expected auto-by-path/from conflict")
	}
}

func TestParseProjectsRenameArgsByID(t *testing.T) {
	opts, err := parseProjectsRenameArgs([]string{"--id=alpha", "--name=Alpha Project", "--dry-run", "--json"})
	if err != nil {
		t.Fatalf("parse projects rename args: %v", err)
	}
	if opts.ID != "alpha" || opts.Path != "" || opts.Name != "Alpha Project" || !opts.DryRun || !opts.JSON || opts.Yes {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseProjectsRenameArgsRejectsAmbiguousSelector(t *testing.T) {
	_, err := parseProjectsRenameArgs([]string{"--id=alpha", "--path=/tmp/alpha", "--name=Alpha", "--dry-run"})
	if err == nil {
		t.Fatal("expected selector conflict")
	}
}

func TestParseProjectsRenameArgsRequiresSafetyFlag(t *testing.T) {
	_, err := parseProjectsRenameArgs([]string{"--id=alpha", "--name=Alpha"})
	if err == nil {
		t.Fatal("expected safety flag error")
	}
}

func TestProjectsListFiltersAndSorts(t *testing.T) {
	projects := []store.ProjectSummary{
		{ID: "beta", Name: "Beta", ObservationCount: 2, LastSeenAt: "2026-01-15 10:00:00"},
		{ID: "alpha", Name: "Alpha", ObservationCount: 0, LastSeenAt: ""},
		{ID: "gamma", Name: "Gamma", ObservationCount: 5, LastSeenAt: "2026-01-10 10:00:00"},
	}
	opts := projectsListOptions{
		SortBy:      "observations",
		Desc:        true,
		Empty:       false,
		UnusedSince: time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
	}

	got := filterProjectsList(projects, opts)
	sortProjectsList(got, opts)

	if len(got) != 2 {
		t.Fatalf("filtered projects = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].ID != "gamma" || got[1].ID != "alpha" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestAutoMergePairingsGroupsByDirectoryAndPrefersUUID(t *testing.T) {
	uuidProject := "11111111-2222-3333-4444-555555555555"
	legacyProject := "projects-alpha"

	projects := []store.ProjectSummary{
		{ID: uuidProject, Directory: "/tmp/alpha", ObservationCount: 1},
		{ID: legacyProject, Directory: "/tmp/alpha", ObservationCount: 1},
	}

	pairings := autoMergePairings(projects)
	if len(pairings) != 1 {
		t.Fatalf("pairings = %d, want 1 (%+v)", len(pairings), pairings)
	}
	if pairings[0][0] != legacyProject || pairings[0][1] != uuidProject {
		t.Fatalf("unexpected pairing (source, dest) = (%s, %s)", pairings[0][0], pairings[0][1])
	}
}

func TestPrintProjectsMergeApplyOutputReportsPartialResults(t *testing.T) {
	source := "projects-alpha"
	destination := "11111111-2222-3333-4444-555555555555"

	results := []store.ProjectMergeResult{
		{
			Plan:                store.ProjectMergePlan{From: store.ProjectSummary{ID: source}, To: store.ProjectSummary{ID: destination}},
			ObservationsUpdated: 1,
		},
	}

	var out bytes.Buffer
	if err := printProjectsMergeApplyOutput(&out, true, results); err != nil {
		t.Fatalf("print partial results: %v", err)
	}
	var report projectsMergeReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal partial results: %v", err)
	}
	if report.Total != 1 || len(report.Results) != 1 {
		t.Fatalf("unexpected partial report: %+v", report)
	}
	if report.Results[0].Plan.From.ID != source || report.Results[0].Plan.To.ID != destination {
		t.Fatalf("unexpected result IDs: %+v", report.Results[0])
	}
}

func TestPreferProjectMergeDestinationPrefersUUIDThenActivity(t *testing.T) {
	uuidProject := store.ProjectSummary{ID: "11111111-2222-3333-4444-555555555555", ObservationCount: 1}
	legacyProject := store.ProjectSummary{ID: "projects-alpha", ObservationCount: 10}
	if !preferProjectMergeDestination(uuidProject, legacyProject) {
		t.Fatal("expected UUID project to be preferred")
	}

	activeProject := store.ProjectSummary{ID: "22222222-3333-4444-5555-666666666666", ObservationCount: 5}
	lessActiveProject := store.ProjectSummary{ID: "11111111-2222-3333-4444-555555555555", ObservationCount: 1}
	if !preferProjectMergeDestination(activeProject, lessActiveProject) {
		t.Fatal("expected more active UUID project to be preferred")
	}
}

func TestPrintProjectsListUsesRenamedProjectName(t *testing.T) {
	projects := []store.ProjectSummary{{
		ID:               "alpha",
		Name:             "Alpha Project",
		Directory:        "/tmp/alpha",
		ObservationCount: 1,
		SessionCount:     2,
		PromptCount:      3,
		LastSeenAt:       "2026-01-15 10:00:00",
	}}
	var out bytes.Buffer

	printProjectsListTo(&out, projects)

	got := out.String()
	if !strings.Contains(got, "Alpha Project") {
		t.Fatalf("table output missing renamed project name:\n%s", got)
	}
	if strings.Contains(got, "/tmp/alpha") {
		t.Fatalf("table output should prefer explicit project name over directory:\n%s", got)
	}
}

func TestPrintProjectsListFallsBackToDirectoryForUnrenamedProject(t *testing.T) {
	projects := []store.ProjectSummary{{
		ID:        "alpha",
		Name:      "alpha",
		Directory: "/tmp/alpha",
	}}
	var out bytes.Buffer

	printProjectsListTo(&out, projects)

	if got := out.String(); !strings.Contains(got, "/tmp/alpha") {
		t.Fatalf("table output missing directory fallback:\n%s", got)
	}
}

func TestProjectsTableCellSanitizesWhitespace(t *testing.T) {
	got := projectsTableCell(" Alpha\nProject\t Demo\r\nSuite  ")
	if got != "Alpha Project Demo Suite" {
		t.Fatalf("sanitized cell = %q, want %q", got, "Alpha Project Demo Suite")
	}
}

func TestPrintProjectsListSanitizesTableOutput(t *testing.T) {
	projects := []store.ProjectSummary{{
		ID:               "project\tone",
		Name:             "Alpha\nProject\t Demo\r\nSuite",
		ObservationCount: 1,
		SessionCount:     2,
		PromptCount:      3,
		LastSeenAt:       "2026-01-15\n10:00:00",
	}}
	var out bytes.Buffer

	printProjectsListTo(&out, projects)

	got := out.String()
	if strings.Contains(got, "Alpha\nProject") || strings.Contains(got, "\t") || strings.Contains(got, "project\tone") {
		t.Fatalf("table output was not sanitized:\n%s", got)
	}
	if !strings.Contains(got, "project one") || !strings.Contains(got, "Alpha Project Demo Suite") || !strings.Contains(got, "2026-01-15 10:00:00") {
		t.Fatalf("table output missing sanitized values:\n%s", got)
	}
}

func TestPrintProjectsListJSONPreservesWhitespace(t *testing.T) {
	project := store.ProjectSummary{
		ID:               "project\tone",
		Name:             "Alpha\nProject\t Demo\r\nSuite",
		CreatedAt:        "2026-01-01\n09:00:00",
		Directory:        "/tmp/alpha\tproject\nsuite",
		ObservationCount: 1,
		SessionCount:     2,
		PromptCount:      3,
		LastSeenAt:       "2026-01-15\n10:00:00",
	}
	var out bytes.Buffer

	if err := printProjectsListJSONTo(&out, []store.ProjectSummary{project}); err != nil {
		t.Fatalf("print projects json: %v", err)
	}

	var got projectsListReport
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal projects json: %v", err)
	}
	if got.Total != 1 || len(got.Projects) != 1 {
		t.Fatalf("unexpected report: %+v", got)
	}
	if got.Projects[0] != project {
		t.Fatalf("json project = %+v, want %+v", got.Projects[0], project)
	}
}

func fixedProjectsNow() time.Time {
	return time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
}
