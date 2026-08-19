package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

type memoriesReviewOptions struct {
	Project  string
	Scope    string
	TopicKey string
	Limit    int
	JSON     bool
}

type memoriesConsolidateTopicOptions struct {
	FromTopic string
	ToTopic   string
	Project   string
	Scope     string
	DryRun    bool
	Yes       bool
	JSON      bool
}

type memoriesConsolidateTopicReport struct {
	DryRun bool                               `json:"dry_run"`
	Plan   store.MemoryTopicConsolidationPlan `json:"plan"`
}

func runMemories(s *store.Store) {
	if len(os.Args) < 3 {
		printMemoriesUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "review":
		runMemoriesReview(s)
	case "mark-reviewed":
		runMemoriesMarkReviewed(s)
	case "mark-stale":
		runMemoriesMarkStale(s)
	case "supersede":
		runMemoriesSupersede(s)
	case "consolidate-topic":
		runMemoriesConsolidateTopic(s)
	default:
		printMemoriesUsage()
		os.Exit(1)
	}
}

func printMemoriesUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo memories review [--project=PROJECT] [--scope=SCOPE] [--topic=TOPIC_KEY] [--limit=N] [--json]")
	fmt.Fprintln(os.Stderr, "       mnemo memories mark-reviewed OBSERVATION_ID [--reason=TEXT]")
	fmt.Fprintln(os.Stderr, "       mnemo memories mark-stale OBSERVATION_ID [--reason=TEXT]")
	fmt.Fprintln(os.Stderr, "       mnemo memories supersede OLD_ID --by=NEW_ID [--reason=TEXT]")
	fmt.Fprintln(os.Stderr, "       mnemo memories consolidate-topic --from=TOPIC_KEY --to=TOPIC_KEY (--dry-run|--yes) [--project=PROJECT] [--scope=SCOPE] [--json]")
}

func runMemoriesReview(s *store.Store) {
	opts, err := parseMemoriesReviewArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories review: %v\n", err)
		os.Exit(1)
	}
	report, err := s.ReviewMemoryConflicts(store.MemoryReviewOptions{
		Project: opts.Project, Scope: opts.Scope, TopicKey: opts.TopicKey, Limit: opts.Limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories review: %v\n", err)
		os.Exit(1)
	}
	if opts.JSON {
		if err := printJSONTo(os.Stdout, report); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo memories review: json: %v\n", err)
			os.Exit(1)
		}
		return
	}
	printMemoriesReviewReport(os.Stdout, report)
}

func runMemoriesMarkReviewed(s *store.Store) {
	id, reason, err := parseMemoryStateArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories mark-reviewed: %v\n", err)
		os.Exit(1)
	}
	if err := s.MarkMemoryReviewed(id, reason); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories mark-reviewed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Marked observation #%d as reviewed\n", id)
}

func runMemoriesMarkStale(s *store.Store) {
	id, reason, err := parseMemoryStateArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories mark-stale: %v\n", err)
		os.Exit(1)
	}
	if err := s.MarkMemoryStale(id, reason); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories mark-stale: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Marked observation #%d as stale\n", id)
}

func runMemoriesSupersede(s *store.Store) {
	oldID, newID, reason, err := parseMemoriesSupersedeArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories supersede: %v\n", err)
		os.Exit(1)
	}
	if err := s.SupersedeMemory(oldID, newID, reason); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories supersede: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Marked observation #%d as superseded by #%d\n", oldID, newID)
}

func runMemoriesConsolidateTopic(s *store.Store) {
	opts, err := parseMemoriesConsolidateTopicArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: %v\n", err)
		os.Exit(1)
	}
	var plan *store.MemoryTopicConsolidationPlan
	if opts.DryRun {
		plan, err = s.PlanMemoryTopicConsolidation(opts.FromTopic, opts.ToTopic, opts.Project, opts.Scope)
	} else {
		plan, err = s.ConsolidateMemoryTopic(opts.FromTopic, opts.ToTopic, opts.Project, opts.Scope)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: %v\n", err)
		os.Exit(1)
	}
	if opts.JSON {
		report := memoriesConsolidateTopicReport{DryRun: opts.DryRun, Plan: *plan}
		if err := printJSONTo(os.Stdout, report); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: json: %v\n", err)
			os.Exit(1)
		}
		return
	}
	printMemoriesConsolidateTopic(os.Stdout, opts.DryRun, plan)
}

func parseMemoriesReviewArgs(args []string) (memoriesReviewOptions, error) {
	opts := memoriesReviewOptions{Limit: 20}
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--project="):
			opts.Project = strings.TrimSpace(arg[len("--project="):])
		case strings.HasPrefix(arg, "--scope="):
			opts.Scope = strings.TrimSpace(arg[len("--scope="):])
		case strings.HasPrefix(arg, "--topic="):
			opts.TopicKey = strings.TrimSpace(arg[len("--topic="):])
		case strings.HasPrefix(arg, "--limit="):
			limit, err := strconv.Atoi(strings.TrimSpace(arg[len("--limit="):]))
			if err != nil || limit < 0 {
				return opts, fmt.Errorf("invalid --limit %q", arg[len("--limit="):])
			}
			opts.Limit = limit
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return opts, nil
}

func parseMemoryStateArgs(args []string) (int64, string, error) {
	if len(args) == 0 {
		return 0, "", fmt.Errorf("observation id is required")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("invalid observation id %q", args[0])
	}
	reason := ""
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--reason=") {
			reason = strings.TrimSpace(arg[len("--reason="):])
			continue
		}
		return 0, "", fmt.Errorf("unknown argument %q", arg)
	}
	return id, reason, nil
}

func parseMemoriesSupersedeArgs(args []string) (int64, int64, string, error) {
	if len(args) == 0 {
		return 0, 0, "", fmt.Errorf("old observation id is required")
	}
	oldID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || oldID <= 0 {
		return 0, 0, "", fmt.Errorf("invalid old observation id %q", args[0])
	}
	var newID int64
	reason := ""
	for _, arg := range args[1:] {
		switch {
		case strings.HasPrefix(arg, "--by="):
			parsed, err := strconv.ParseInt(strings.TrimSpace(arg[len("--by="):]), 10, 64)
			if err != nil || parsed <= 0 {
				return 0, 0, "", fmt.Errorf("invalid --by id %q", arg[len("--by="):])
			}
			newID = parsed
		case strings.HasPrefix(arg, "--reason="):
			reason = strings.TrimSpace(arg[len("--reason="):])
		default:
			return 0, 0, "", fmt.Errorf("unknown argument %q", arg)
		}
	}
	if newID == 0 {
		return 0, 0, "", fmt.Errorf("--by is required")
	}
	return oldID, newID, reason, nil
}

func parseMemoriesConsolidateTopicArgs(args []string) (memoriesConsolidateTopicOptions, error) {
	var opts memoriesConsolidateTopicOptions
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--yes":
			opts.Yes = true
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--from="):
			opts.FromTopic = strings.TrimSpace(arg[len("--from="):])
		case strings.HasPrefix(arg, "--to="):
			opts.ToTopic = strings.TrimSpace(arg[len("--to="):])
		case strings.HasPrefix(arg, "--project="):
			opts.Project = strings.TrimSpace(arg[len("--project="):])
		case strings.HasPrefix(arg, "--scope="):
			opts.Scope = strings.TrimSpace(arg[len("--scope="):])
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.FromTopic == "" || opts.ToTopic == "" {
		return opts, fmt.Errorf("--from and --to are required")
	}
	if opts.DryRun && opts.Yes {
		return opts, fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !opts.DryRun && !opts.Yes {
		return opts, fmt.Errorf("refusing to consolidate without --dry-run or --yes")
	}
	return opts, nil
}

func printMemoriesReviewReport(w io.Writer, report *store.MemoryReviewReport) {
	if report.Total == 0 {
		_, _ = fmt.Fprintln(w, "No potential memory conflicts found.")
		return
	}
	_, _ = fmt.Fprintf(w, "Found %d potential memory conflict(s):\n\n", report.Total)
	for i, group := range report.Groups {
		_, _ = fmt.Fprintf(w, "%d. %s (confidence %.2f)\n", i+1, group.Kind, group.Confidence)
		_, _ = fmt.Fprintf(w, "   Reason: %s\n", group.Reason)
		if group.TopicKey != "" {
			_, _ = fmt.Fprintf(w, "   Topic: %s\n", group.TopicKey)
		}
		_, _ = fmt.Fprintf(w, "   Suggested action: %s\n", group.SuggestedAction)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "   ID\tTYPE\tTITLE\tUPDATED")
		for _, obs := range group.Observations {
			_, _ = fmt.Fprintf(tw, "   %d\t%s\t%s\t%s\n", obs.ID, obs.Type, obs.Title, obs.UpdatedAt)
		}
		_ = tw.Flush()
		_, _ = fmt.Fprintln(w)
	}
}

func printMemoriesConsolidateTopic(w io.Writer, dryRun bool, plan *store.MemoryTopicConsolidationPlan) {
	verb := "Would update"
	if !dryRun {
		verb = "Updated"
	}
	_, _ = fmt.Fprintf(w, "%s %d observation(s) from topic %q to %q\n", verb, plan.Total, plan.FromTopic, plan.ToTopic)
	if len(plan.IDs) > 0 {
		parts := make([]string, 0, len(plan.IDs))
		for _, id := range plan.IDs {
			parts = append(parts, fmt.Sprintf("#%d", id))
		}
		_, _ = fmt.Fprintf(w, "Observations: %s\n", strings.Join(parts, ", "))
	}
}

func printJSONTo(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
