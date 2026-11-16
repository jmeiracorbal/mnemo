package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo/v2/protocol"
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
	Plan   protocol.MemoryConsolidationOutput `json:"plan"`
}

func runMemories() {
	if len(os.Args) < 3 {
		printMemoriesUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "review":
		runMemoriesReview()
	case "mark-reviewed":
		runMemoriesMarkReviewed()
	case "mark-stale":
		runMemoriesMarkStale()
	case "supersede":
		runMemoriesSupersede()
	case "consolidate-topic":
		runMemoriesConsolidateTopic()
	default:
		printMemoriesUsage()
		os.Exit(1)
	}
}

func printMemoriesUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo memories review [--project=PROJECT] [--scope=SCOPE] [--topic=TOPIC_KEY] [--limit=N] [--json]")
	fmt.Fprintln(os.Stderr, "       mnemo memories mark-reviewed MNEMO_REF [--reason=TEXT]")
	fmt.Fprintln(os.Stderr, "       mnemo memories mark-stale MNEMO_REF [--reason=TEXT]")
	fmt.Fprintln(os.Stderr, "       mnemo memories supersede OLD_REF --by=NEW_REF [--reason=TEXT]")
	fmt.Fprintln(os.Stderr, "       mnemo memories consolidate-topic --from=TOPIC_KEY --to=TOPIC_KEY (--dry-run|--yes) [--project=PROJECT] [--scope=SCOPE] [--json]")
}

func runMemoriesReview() {
	opts, err := parseMemoriesReviewArgs(os.Args[3:])
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories review: %v\n", err); os.Exit(1) }
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories review: %v\n", err); os.Exit(1) }
	report, err := api.ReviewMemories(context.Background(), protocol.MemoryReviewInput{
		ProjectID: opts.Project, Scope: opts.Scope, TopicKey: opts.TopicKey, Limit: opts.Limit,
	})
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories review: %v\n", err); os.Exit(1) }
	if opts.JSON {
		if err := printJSONTo(os.Stdout, report); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo memories review: json: %v\n", err)
			os.Exit(1)
		}
		return
	}
	printMemoriesReviewReport(os.Stdout, &report)
}

func runMemoriesMarkReviewed() {
	ref, reason, err := parseMemoryStateArgs(os.Args[3:])
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories mark-reviewed: %v\n", err); os.Exit(1) }
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories mark-reviewed: %v\n", err); os.Exit(1) }
	if err := api.MarkMemoryReviewed(context.Background(), uuid.NewString(), protocol.MemoryMarkInput{Ref: ref, Reason: reason}); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories mark-reviewed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Marked observation %s as reviewed\n", ref.String())
}

func runMemoriesMarkStale() {
	ref, reason, err := parseMemoryStateArgs(os.Args[3:])
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories mark-stale: %v\n", err); os.Exit(1) }
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories mark-stale: %v\n", err); os.Exit(1) }
	if err := api.MarkMemoryStale(context.Background(), uuid.NewString(), protocol.MemoryMarkInput{Ref: ref, Reason: reason}); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories mark-stale: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Marked observation %s as stale\n", ref.String())
}

func runMemoriesSupersede() {
	oldRef, newRef, reason, err := parseMemoriesSupersedeArgs(os.Args[3:])
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories supersede: %v\n", err); os.Exit(1) }
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories supersede: %v\n", err); os.Exit(1) }
	if err := api.SupersedeMemory(context.Background(), uuid.NewString(), protocol.MemorySupersedeInput{
		OldRef: oldRef, NewRef: newRef, Reason: reason,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo memories supersede: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Marked observation %s as superseded by %s\n", oldRef.String(), newRef.String())
}

func runMemoriesConsolidateTopic() {
	opts, err := parseMemoriesConsolidateTopicArgs(os.Args[3:])
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: %v\n", err); os.Exit(1) }
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: %v\n", err); os.Exit(1) }
	input := protocol.MemoryConsolidationInput{
		FromTopic: opts.FromTopic, ToTopic: opts.ToTopic,
		ProjectID: opts.Project, Scope: opts.Scope,
	}
	var plan protocol.MemoryConsolidationOutput
	if opts.DryRun {
		plan, err = api.PlanMemoryConsolidation(context.Background(), input)
	} else {
		plan, err = api.ConsolidateMemories(context.Background(), uuid.NewString(), input)
	}
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: %v\n", err); os.Exit(1) }
	if opts.JSON {
		if err := printJSONTo(os.Stdout, memoriesConsolidateTopicReport{DryRun: opts.DryRun, Plan: plan}); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo memories consolidate-topic: json: %v\n", err)
			os.Exit(1)
		}
		return
	}
	printMemoriesConsolidateTopic(os.Stdout, opts.DryRun, &plan)
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

func parseMemoryStateArgs(args []string) (protocol.ResourceRef, string, error) {
	if len(args) == 0 { return protocol.ResourceRef{}, "", fmt.Errorf("observation reference is required") }
	ref, err := protocol.ParseResourceRef(args[0])
	if err != nil { return protocol.ResourceRef{}, "", err }
	reason := ""
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--reason=") {
			reason = strings.TrimSpace(arg[len("--reason="):])
			continue
		}
		return protocol.ResourceRef{}, "", fmt.Errorf("unknown argument %q", arg)
	}
	return ref, reason, nil
}

func parseMemoriesSupersedeArgs(args []string) (protocol.ResourceRef, protocol.ResourceRef, string, error) {
	if len(args) == 0 { return protocol.ResourceRef{}, protocol.ResourceRef{}, "", fmt.Errorf("old observation reference is required") }
	oldRef, err := protocol.ParseResourceRef(args[0])
	if err != nil { return protocol.ResourceRef{}, protocol.ResourceRef{}, "", err }
	var newRef protocol.ResourceRef
	reason := ""
	for _, arg := range args[1:] {
		switch {
		case strings.HasPrefix(arg, "--by="):
			newRef, err = protocol.ParseResourceRef(strings.TrimSpace(arg[len("--by="):]))
			if err != nil { return protocol.ResourceRef{}, protocol.ResourceRef{}, "", err }
		case strings.HasPrefix(arg, "--reason="):
			reason = strings.TrimSpace(arg[len("--reason="):])
		default:
			return protocol.ResourceRef{}, protocol.ResourceRef{}, "", fmt.Errorf("unknown argument %q", arg)
		}
	}
	if err := newRef.Validate(); err != nil { return protocol.ResourceRef{}, protocol.ResourceRef{}, "", fmt.Errorf("--by requires a valid Mnemo reference: %w", err) }
	return oldRef, newRef, reason, nil
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

func printMemoriesReviewReport(w io.Writer, report *protocol.MemoryReviewOutput) {
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
		_, _ = fmt.Fprintln(tw, "   REF\tTYPE\tTITLE\tUPDATED")
		for _, obs := range group.Observations {
			_, _ = fmt.Fprintf(tw, "   %s\t%s\t%s\t%s\n", obs.Ref.String(), obs.Type, obs.Title, obs.UpdatedAt)
		}
		_ = tw.Flush()
		_, _ = fmt.Fprintln(w)
	}
}

func printMemoriesConsolidateTopic(w io.Writer, dryRun bool, plan *protocol.MemoryConsolidationOutput) {
	verb := "Would update"
	if !dryRun {
		verb = "Updated"
	}
	_, _ = fmt.Fprintf(w, "%s %d observation(s) from topic %q to %q\n", verb, plan.Total, plan.FromTopic, plan.ToTopic)
	if len(plan.Refs) > 0 {
		parts := make([]string, 0, len(plan.Refs))
		for _, id := range plan.Refs {
			parts = append(parts, id.String())
		}
		_, _ = fmt.Fprintf(w, "Observations: %s\n", strings.Join(parts, ", "))
	}
}

func printJSONTo(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
