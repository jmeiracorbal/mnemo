package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

type projectsListOptions struct {
	SortBy      string
	Desc        bool
	JSON        bool
	Empty       bool
	UnusedSince time.Time
}

type projectsListReport struct {
	Total    int                    `json:"total"`
	Projects []store.ProjectSummary `json:"projects"`
}

func runProjects(s *store.Store) {
	if len(os.Args) < 3 {
		printProjectsUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "list":
		runProjectsList(s)
	default:
		printProjectsUsage()
		os.Exit(1)
	}
}

func printProjectsUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo projects list [--sort=FIELD] [--asc|--desc] [--unused-since=DURATION|DATE] [--empty] [--json]")
}

func runProjectsList(s *store.Store) {
	opts, err := parseProjectsListArgs(os.Args[3:], time.Now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects list: %v\n", err)
		os.Exit(1)
	}
	projects, err := buildProjectsList(s, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects list: %v\n", err)
		os.Exit(1)
	}
	if opts.JSON {
		if err := printProjectsListJSONTo(os.Stdout, projects); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo projects list: json: %v\n", err)
			os.Exit(1)
		}
		return
	}
	printProjectsList(projects)
}

func parseProjectsListArgs(args []string, now func() time.Time) (projectsListOptions, error) {
	opts := projectsListOptions{SortBy: "last-seen", Desc: true}
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--empty":
			opts.Empty = true
		case arg == "--asc":
			opts.Desc = false
		case arg == "--desc":
			opts.Desc = true
		case strings.HasPrefix(arg, "--sort="):
			sortBy, err := normalizeProjectsSort(arg[len("--sort="):])
			if err != nil {
				return opts, err
			}
			opts.SortBy = sortBy
		case strings.HasPrefix(arg, "--unused-since="):
			cutoff, err := parseUnusedSince(arg[len("--unused-since="):], now())
			if err != nil {
				return opts, err
			}
			opts.UnusedSince = cutoff
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return opts, nil
}

func normalizeProjectsSort(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "id", "name", "observations", "last-seen":
		return strings.TrimSpace(value), nil
	case "last_seen", "lastSeen":
		return "last-seen", nil
	default:
		return "", fmt.Errorf("unsupported sort %q — valid: id | name | observations | last-seen", value)
	}
}

func parseUnusedSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("--unused-since requires a duration or date")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days < 0 {
			return time.Time{}, fmt.Errorf("invalid --unused-since %q", value)
		}
		return now.Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration < 0 {
			return time.Time{}, fmt.Errorf("invalid --unused-since %q", value)
		}
		return now.Add(-duration), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --unused-since %q", value)
}

func buildProjectsList(s *store.Store, opts projectsListOptions) ([]store.ProjectSummary, error) {
	projects, err := s.ListProjectSummaries()
	if err != nil {
		return nil, err
	}
	projects = filterProjectsList(projects, opts)
	sortProjectsList(projects, opts)
	return projects, nil
}

func filterProjectsList(projects []store.ProjectSummary, opts projectsListOptions) []store.ProjectSummary {
	filtered := projects[:0]
	for _, project := range projects {
		if opts.Empty && (project.ObservationCount != 0 || project.PromptCount != 0) {
			continue
		}
		if !opts.UnusedSince.IsZero() && !projectUnusedSince(project, opts.UnusedSince) {
			continue
		}
		filtered = append(filtered, project)
	}
	return filtered
}

func projectUnusedSince(project store.ProjectSummary, cutoff time.Time) bool {
	if strings.TrimSpace(project.LastSeenAt) == "" {
		return true
	}
	lastSeen, err := parseProjectTime(project.LastSeenAt)
	if err != nil {
		return project.LastSeenAt < cutoff.Format("2006-01-02 15:04:05")
	}
	return lastSeen.Before(cutoff)
}

func sortProjectsList(projects []store.ProjectSummary, opts projectsListOptions) {
	sort.SliceStable(projects, func(i, j int) bool {
		cmp := compareProjects(projects[i], projects[j], opts.SortBy)
		if cmp == 0 {
			return projects[i].ID < projects[j].ID
		}
		if opts.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareProjects(a, b store.ProjectSummary, sortBy string) int {
	switch sortBy {
	case "id":
		return strings.Compare(a.ID, b.ID)
	case "name":
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "observations":
		return compareInt(a.ObservationCount, b.ObservationCount)
	case "last-seen":
		return compareLastSeen(a.LastSeenAt, b.LastSeenAt)
	default:
		return 0
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareLastSeen(a, b string) int {
	if strings.TrimSpace(a) == "" && strings.TrimSpace(b) == "" {
		return 0
	}
	if strings.TrimSpace(a) == "" {
		return -1
	}
	if strings.TrimSpace(b) == "" {
		return 1
	}
	at, aerr := parseProjectTime(a)
	bt, berr := parseProjectTime(b)
	if aerr == nil && berr == nil {
		switch {
		case at.Before(bt):
			return -1
		case at.After(bt):
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(a, b)
}

func parseProjectTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid project timestamp %q", value)
}

func printProjectsList(projects []store.ProjectSummary) {
	printProjectsListTo(os.Stdout, projects)
}

func printProjectsListJSONTo(out io.Writer, projects []store.ProjectSummary) error {
	data, err := json.MarshalIndent(projectsListReport{Total: len(projects), Projects: projects}, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func printProjectsListTo(out io.Writer, projects []store.ProjectSummary) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tName/Path\tObservations\tSessions\tPrompts\tLast Seen")
	for _, project := range projects {
		lastSeen := project.LastSeenAt
		if strings.TrimSpace(lastSeen) == "" {
			lastSeen = "never"
		}
		nameOrPath := project.Name
		if strings.TrimSpace(project.Directory) != "" {
			nameOrPath = project.Directory
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%s\n",
			projectsTableCell(project.ID),
			projectsTableCell(nameOrPath),
			project.ObservationCount,
			project.SessionCount,
			project.PromptCount,
			projectsTableCell(lastSeen),
		)
	}
	_ = w.Flush()
}

func projectsTableCell(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
