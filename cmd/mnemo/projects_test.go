package main

import (
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

func fixedProjectsNow() time.Time {
	return time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
}
