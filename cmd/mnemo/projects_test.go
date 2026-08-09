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
