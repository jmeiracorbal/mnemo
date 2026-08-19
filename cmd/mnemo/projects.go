package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type projectsMergeOptions struct {
	From       string
	To         string
	AutoByPath bool
	DryRun     bool
	Yes        bool
	JSON       bool
}

type projectsRenameOptions struct {
	ID     string
	Path   string
	Name   string
	DryRun bool
	Yes    bool
	JSON   bool
}

type projectsMergeReport struct {
	DryRun  bool                       `json:"dry_run"`
	Total   int                        `json:"total"`
	Plans   []store.ProjectMergePlan   `json:"plans,omitempty"`
	Results []store.ProjectMergeResult `json:"results,omitempty"`
}

type projectsRenameReport struct {
	DryRun bool                       `json:"dry_run"`
	Plan   *store.ProjectRenamePlan   `json:"plan,omitempty"`
	Result *store.ProjectRenameResult `json:"result,omitempty"`
}

func runProjects(s *store.Store) {
	if len(os.Args) < 3 {
		printProjectsUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "list":
		runProjectsList(s)
	case "merge":
		runProjectsMerge(s)
	case "rename":
		runProjectsRename(s)
	default:
		printProjectsUsage()
		os.Exit(1)
	}
}

func printProjectsUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo projects list [--sort=FIELD] [--asc|--desc] [--unused-since=DURATION|DATE] [--empty] [--json]")
	fmt.Fprintln(os.Stderr, "       mnemo projects merge --from=PROJECT --to=PROJECT (--dry-run|--yes) [--json]")
	fmt.Fprintln(os.Stderr, "       mnemo projects merge --auto-by-path (--dry-run|--yes) [--json]")
	fmt.Fprintln(os.Stderr, "       mnemo projects rename (--id=PROJECT|--path=DIR) --name=NAME (--dry-run|--yes) [--json]")
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

func runProjectsMerge(s *store.Store) {
	opts, err := parseProjectsMergeArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects merge: %v\n", err)
		os.Exit(1)
	}
	plans, err := buildProjectsMergePlans(s, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects merge: %v\n", err)
		os.Exit(1)
	}
	if opts.DryRun {
		if opts.JSON {
			if err := printProjectsMergeJSONTo(os.Stdout, projectsMergeReport{DryRun: true, Total: len(plans), Plans: plans}); err != nil {
				fmt.Fprintf(os.Stderr, "mnemo projects merge: json: %v\n", err)
				os.Exit(1)
			}
			return
		}
		printProjectsMergePlans(os.Stdout, plans)
		return
	}

	results, err := applyProjectsMergePlans(s, plans)
	if err != nil {
		if len(results) > 0 {
			if emitErr := printProjectsMergeApplyOutput(os.Stdout, opts.JSON, results); emitErr != nil {
				fmt.Fprintf(os.Stderr, "mnemo projects merge: json: %v\n", emitErr)
			}
		}
		fmt.Fprintf(os.Stderr, "mnemo projects merge: %v\n", err)
		os.Exit(1)
	}
	if err := printProjectsMergeApplyOutput(os.Stdout, opts.JSON, results); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects merge: json: %v\n", err)
		os.Exit(1)
	}
}

func runProjectsRename(s *store.Store) {
	opts, err := parseProjectsRenameArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects rename: %v\n", err)
		os.Exit(1)
	}
	selector := projectsRenameSelector(opts)
	if opts.DryRun {
		plan, err := s.BuildProjectRenamePlan(selector, opts.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo projects rename: %v\n", err)
			os.Exit(1)
		}
		if err := printProjectsRenameDryRunOutput(os.Stdout, opts.JSON, *plan); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo projects rename: json: %v\n", err)
			os.Exit(1)
		}
		return
	}
	result, err := s.RenameProject(selector, opts.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects rename: %v\n", err)
		os.Exit(1)
	}
	if err := printProjectsRenameApplyOutput(os.Stdout, opts.JSON, *result); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo projects rename: json: %v\n", err)
		os.Exit(1)
	}
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

func parseProjectsMergeArgs(args []string) (projectsMergeOptions, error) {
	var opts projectsMergeOptions
	for _, arg := range args {
		switch {
		case arg == "--auto-by-path":
			opts.AutoByPath = true
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--yes":
			opts.Yes = true
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--from="):
			opts.From = strings.TrimSpace(arg[len("--from="):])
		case strings.HasPrefix(arg, "--to="):
			opts.To = strings.TrimSpace(arg[len("--to="):])
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.DryRun && opts.Yes {
		return opts, fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !opts.DryRun && !opts.Yes {
		return opts, fmt.Errorf("refusing to merge without --dry-run or --yes")
	}
	if opts.AutoByPath {
		if opts.From != "" || opts.To != "" {
			return opts, fmt.Errorf("--auto-by-path cannot be combined with --from or --to")
		}
		return opts, nil
	}
	if opts.From == "" || opts.To == "" {
		return opts, fmt.Errorf("explicit merge requires --from and --to")
	}
	return opts, nil
}

func parseProjectsRenameArgs(args []string) (projectsRenameOptions, error) {
	var opts projectsRenameOptions
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--yes":
			opts.Yes = true
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--id="):
			opts.ID = strings.TrimSpace(arg[len("--id="):])
		case strings.HasPrefix(arg, "--path="):
			opts.Path = strings.TrimSpace(arg[len("--path="):])
		case strings.HasPrefix(arg, "--name="):
			opts.Name = strings.TrimSpace(arg[len("--name="):])
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.DryRun && opts.Yes {
		return opts, fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !opts.DryRun && !opts.Yes {
		return opts, fmt.Errorf("refusing to rename without --dry-run or --yes")
	}
	if (opts.ID == "") == (opts.Path == "") {
		return opts, fmt.Errorf("select exactly one project with --id or --path")
	}
	if opts.Name == "" {
		return opts, fmt.Errorf("--name is required")
	}
	return opts, nil
}

func projectsRenameSelector(opts projectsRenameOptions) store.ProjectRenameSelector {
	return store.ProjectRenameSelector{ID: opts.ID, Path: opts.Path}
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

func buildProjectsMergePlans(s *store.Store, opts projectsMergeOptions) ([]store.ProjectMergePlan, error) {
	if !opts.AutoByPath {
		plan, err := s.BuildProjectMergePlan(opts.From, opts.To)
		if err != nil {
			return nil, err
		}
		return []store.ProjectMergePlan{*plan}, nil
	}

	projects, err := s.ListProjectSummaries()
	if err != nil {
		return nil, err
	}
	groups := make(map[string][]store.ProjectSummary)
	for _, project := range projects {
		directory := normalizedProjectDirectory(project.Directory)
		if directory == "" {
			continue
		}
		groups[directory] = append(groups[directory], project)
	}
	directories := make([]string, 0, len(groups))
	for directory, group := range groups {
		if len(group) > 1 {
			directories = append(directories, directory)
		}
	}
	sort.Strings(directories)

	var plans []store.ProjectMergePlan
	for _, directory := range directories {
		group := groups[directory]
		sort.SliceStable(group, func(i, j int) bool {
			return preferProjectMergeDestination(group[i], group[j])
		})
		destination := group[0]
		for _, source := range group[1:] {
			plan, err := s.BuildProjectMergePlan(source.ID, destination.ID)
			if err != nil {
				return nil, err
			}
			plans = append(plans, *plan)
		}
	}
	return plans, nil
}

func applyProjectsMergePlans(s *store.Store, plans []store.ProjectMergePlan) ([]store.ProjectMergeResult, error) {
	results := make([]store.ProjectMergeResult, 0, len(plans))
	for _, plan := range plans {
		result, err := s.MergeProjects(plan.From.ID, plan.To.ID)
		if err != nil {
			return results, fmt.Errorf("%s -> %s: %w", plan.From.ID, plan.To.ID, err)
		}
		results = append(results, *result)
	}
	return results, nil
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

func normalizedProjectDirectory(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return ""
	}
	return filepath.Clean(directory)
}

func preferProjectMergeDestination(a, b store.ProjectSummary) bool {
	aUUID := isUUIDProjectID(a.ID)
	bUUID := isUUIDProjectID(b.ID)
	if aUUID != bUUID {
		return aUUID
	}
	aActivity := projectActivityTotal(a)
	bActivity := projectActivityTotal(b)
	if aActivity != bActivity {
		return aActivity > bActivity
	}
	lastSeen := compareLastSeen(a.LastSeenAt, b.LastSeenAt)
	if lastSeen != 0 {
		return lastSeen > 0
	}
	return a.ID < b.ID
}

func projectActivityTotal(project store.ProjectSummary) int {
	return project.ObservationCount + project.SessionCount + project.PromptCount
}

func isUUIDProjectID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return false
			}
		}
	}
	return true
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

func printProjectsMergeJSONTo(out io.Writer, report projectsMergeReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func printProjectsRenameJSONTo(out io.Writer, report projectsRenameReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func printProjectsMergeApplyOutput(out io.Writer, jsonOutput bool, results []store.ProjectMergeResult) error {
	if jsonOutput {
		return printProjectsMergeJSONTo(out, projectsMergeReport{DryRun: false, Total: len(results), Results: results})
	}
	printProjectsMergeResults(out, results)
	return nil
}

func printProjectsRenameDryRunOutput(out io.Writer, jsonOutput bool, plan store.ProjectRenamePlan) error {
	if jsonOutput {
		return printProjectsRenameJSONTo(out, projectsRenameReport{DryRun: true, Plan: &plan})
	}
	printProjectsRenamePlan(out, plan)
	return nil
}

func printProjectsRenameApplyOutput(out io.Writer, jsonOutput bool, result store.ProjectRenameResult) error {
	if jsonOutput {
		return printProjectsRenameJSONTo(out, projectsRenameReport{DryRun: false, Result: &result})
	}
	printProjectsRenameResult(out, result)
	return nil
}

func printProjectsListTo(out io.Writer, projects []store.ProjectSummary) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tName/Path\tObservations\tSessions\tPrompts\tLast Seen")
	for _, project := range projects {
		lastSeen := project.LastSeenAt
		if strings.TrimSpace(lastSeen) == "" {
			lastSeen = "never"
		}
		nameOrPath := projectDisplayName(project)
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

func projectDisplayName(project store.ProjectSummary) string {
	if strings.TrimSpace(project.Name) != "" && project.Name != project.ID {
		return project.Name
	}
	if strings.TrimSpace(project.Directory) != "" {
		return project.Directory
	}
	return project.Name
}

func printProjectsMergePlans(out io.Writer, plans []store.ProjectMergePlan) {
	if len(plans) == 0 {
		_, _ = fmt.Fprintln(out, "No project merge candidates found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "From\tTo\tObservations\tSessions\tPrompts\tSync\tEnrollment")
	for _, plan := range plans {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
			projectsTableCell(plan.From.ID),
			projectsTableCell(plan.To.ID),
			plan.Observations,
			plan.Sessions,
			plan.Prompts,
			plan.SyncMutations,
			projectMergeEnrollmentCell(plan),
		)
	}
	_, _ = fmt.Fprintln(w, "\nDry run only. Re-run with --yes to apply.")
	_ = w.Flush()
}

func printProjectsRenamePlan(out io.Writer, plan store.ProjectRenamePlan) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tCurrent Name\tNew Name\tSelector\tChange")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s=%s\t%s\n",
		projectsTableCell(plan.Project.ID),
		projectsTableCell(plan.Project.Name),
		projectsTableCell(plan.NewName),
		projectsTableCell(plan.Selector),
		projectsTableCell(plan.SelectorValue),
		projectRenameChangeCell(plan.WillChange),
	)
	_, _ = fmt.Fprintln(w, "\nDry run only. Re-run with --yes to apply.")
	_ = w.Flush()
}

func printProjectsRenameResult(out io.Writer, result store.ProjectRenameResult) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tOld Name\tNew Name\tChanged")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
		projectsTableCell(result.Plan.Project.ID),
		projectsTableCell(result.Plan.Project.Name),
		projectsTableCell(result.Plan.NewName),
		projectRenameChangeCell(result.Renamed),
	)
	_ = w.Flush()
}

func printProjectsMergeResults(out io.Writer, results []store.ProjectMergeResult) {
	if len(results) == 0 {
		_, _ = fmt.Fprintln(out, "No project merge candidates found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "From\tTo\tObservations\tSessions\tPrompts\tSync\tSource Project")
	for _, result := range results {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
			projectsTableCell(result.Plan.From.ID),
			projectsTableCell(result.Plan.To.ID),
			result.ObservationsUpdated,
			result.SessionsUpdated,
			result.PromptsUpdated,
			result.SyncMutationsUpdated,
			projectMergeDeletedCell(result.SourceProjectDeleted),
		)
	}
	_ = w.Flush()
}

func projectMergeEnrollmentCell(plan store.ProjectMergePlan) string {
	switch {
	case plan.WillCopyEnrollment:
		return "copy"
	case plan.SourceEnrolled:
		return "already"
	default:
		return "-"
	}
}

func projectMergeDeletedCell(deleted bool) string {
	if deleted {
		return "deleted"
	}
	return "-"
}

func projectRenameChangeCell(changed bool) string {
	if changed {
		return "yes"
	}
	return "no"
}

func projectsTableCell(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
