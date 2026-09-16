package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/jmeiracorbal/mnemo/internal/events"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

func runSave(s *store.Store) {
	args := os.Args[2:]
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mnemo save <title> <content> --session SESSION --project PROJECT --dir DIR [--type TYPE] [--scope SCOPE] [--topic TOPIC_KEY]")
		os.Exit(1)
	}

	title := args[0]
	content := args[1]
	typ := "manual"
	project := ""
	dir := ""
	scope := ""
	topicKey := ""
	sessionID := ""

	for i := 2; i < len(args)-1; i++ {
		switch args[i] {
		case "--type":
			typ = args[i+1]
			i++
		case "--project":
			project = args[i+1]
			i++
		case "--dir":
			dir = args[i+1]
			i++
		case "--scope":
			scope = args[i+1]
			i++
		case "--topic":
			topicKey = args[i+1]
			i++
		case "--session":
			sessionID = args[i+1]
			i++
		}
	}

	if project == "" {
		fmt.Fprintln(os.Stderr, "mnemo: --project is required")
		os.Exit(1)
	}
	if dir == "" {
		fmt.Fprintln(os.Stderr, "mnemo: --dir is required")
		os.Exit(1)
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "mnemo: --session is required")
		os.Exit(1)
	}

	provenance := store.CLIProvenance(store.ToolMnemoSave)
	if err := s.EnsureSession(sessionID, project, dir); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: could not ensure session: %v\n", err)
		os.Exit(1)
	}

	id, err := s.AddObservation(store.AddObservationParams{
		SessionID:  sessionID,
		Type:       typ,
		Title:      title,
		Content:    content,
		Project:    project,
		Scope:      scope,
		TopicKey:   topicKey,
		Provenance: provenance,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: save failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Memory saved: #%d %q (%s)\n", id, title, typ)
}

// runSearch routes through the controller. It deliberately does not open a
// Store: hook processes must never touch SQLite, even to initialize it.
func runSearch() {
	args := os.Args[2:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mnemo search <query> [--type TYPE] [--project PROJECT] [--scope SCOPE] [--limit N]")
		os.Exit(1)
	}

	query := args[0]
	opts := store.SearchOptions{Limit: 10}

	for i := 1; i < len(args)-1; i++ {
		switch args[i] {
		case "--type":
			opts.Type = args[i+1]
			i++
		case "--project":
			opts.Project = args[i+1]
			i++
		case "--scope":
			opts.Scope = args[i+1]
			i++
		case "--limit":
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				opts.Limit = n
			}
			i++
		}
	}

	cfg, err := loadEventsConfig()
	if err != nil {
		fmt.Printf("No memories found for: %q\n", query)
		return
	}

	var results []store.SearchResult
	input := struct {
		Query   string              `json:"query"`
		Options store.SearchOptions `json:"options"`
	}{query, opts}
	if err := events.Call(context.Background(), cfg, "search", input, &results); err != nil {
		fmt.Printf("No memories found for: %q\n", query)
		return
	}

	if len(results) == 0 {
		fmt.Printf("No memories found for: %q\n", query)
		return
	}

	fmt.Printf("Found %d memories:\n\n", len(results))
	for i, r := range results {
		proj := ""
		if r.Project != nil {
			proj = " | project: " + *r.Project
		}
		content := r.Content
		if len(content) > 300 {
			content = content[:300] + "... [truncated — use mem_get_observation(id) for full content]"
		}
		fmt.Printf("[%d] #%d (%s) — %s\n    %s\n    %s%s | scope: %s\n\n",
			i+1, r.ID, r.Type, r.Title, content, r.CreatedAt, proj, r.Scope)
	}
}

// runContext routes through the controller. It deliberately does not open a
// Store: hook processes must never touch SQLite, even to initialize it.
func runContext() {
	project := ""
	if len(os.Args) > 2 {
		project = os.Args[2]
	}

	cfg, err := loadEventsConfig()
	if err != nil {
		fmt.Println("No previous session memories found.")
		return
	}

	var output string
	input := struct {
		Project string               `json:"project"`
		Scope   string               `json:"scope"`
		Options store.ContextOptions `json:"options"`
	}{project, "", store.ContextOptions{}}
	if err := events.Call(context.Background(), cfg, "format_context", input, &output); err != nil {
		fmt.Println("No previous session memories found.")
		return
	}

	if output == "" {
		fmt.Println("No previous session memories found.")
		return
	}
	fmt.Println(output)
}

func loadEventsConfig() (events.Config, error) {
	storeConfig, err := store.DefaultConfig()
	if err != nil {
		return events.Config{}, err
	}
	return events.LoadConfig(storeConfig.DataDir)
}

func runSession(s *store.Store) {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: mnemo session start <id> [--project PROJECT] [--dir DIR]")
		fmt.Fprintln(os.Stderr, "       mnemo session compact <id>")
		fmt.Fprintln(os.Stderr, "       mnemo session end <id> [--summary SUMMARY]")
		os.Exit(1)
	}

	subcmd := os.Args[2]
	id := os.Args[3]
	args := os.Args[4:]

	switch subcmd {
	case "start":
		project := ""
		dir := ""
		for i := 0; i < len(args)-1; i++ {
			switch args[i] {
			case "--project":
				project = args[i+1]
				i++
			case "--dir":
				dir = args[i+1]
				i++
			}
		}
		if err := s.CreateSession(id, project, dir); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: session start failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Session %q started\n", id)

	case "compact":
		if err := s.TouchCompact(id); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: session compact failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Session %q compact recorded\n", id)

	case "end":
		summary := ""
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--summary" {
				summary = args[i+1]
				i++
			}
		}
		if err := s.EndSession(id, summary); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: session end failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Session %q completed\n", id)

	case "exists":
		_, err := s.GetSession(id)
		if err != nil {
			fmt.Println("false")
			os.Exit(1)
		}
		fmt.Println("true")

	case "obs-count":
		n, err := s.ObsCount(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: obs-count failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(n)

	case "project-obs-count":
		n, err := s.ObsCountForSession(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: project-obs-count failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(n)

	default:
		fmt.Fprintf(os.Stderr, "mnemo: unknown session subcommand %q\n", subcmd)
		os.Exit(1)
	}
}

func runStats(s *store.Store) {
	stats, err := s.Stats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: stats failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Memory stats:\n")
	fmt.Printf("  Sessions:     %d\n", stats.TotalSessions)
	fmt.Printf("  Observations: %d\n", stats.TotalObservations)
	fmt.Printf("  Prompts:      %d\n", stats.TotalPrompts)
	if len(stats.Projects) > 0 {
		fmt.Printf("  Projects:     %v\n", stats.Projects)
	}
}

func runExport(s *store.Store) {
	data, err := s.Export()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: export failed: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: json marshal failed: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 2 {
		if err := os.WriteFile(os.Args[2], out, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: write failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported to %s\n", os.Args[2])
	} else {
		fmt.Println(string(out))
	}
}

func runImport(s *store.Store) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo import <file.json>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: read failed: %v\n", err)
		os.Exit(1)
	}

	var payload store.ExportData
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: invalid json: %v\n", err)
		os.Exit(1)
	}

	result, err := s.Import(&payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: import failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Import complete: %d sessions, %d observations\n", result.SessionsImported, result.ObservationsImported)
}

func runCapture(s *store.Store) {
	args := os.Args[2:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mnemo capture <content>|- --session SESSION --project PROJECT --dir DIR")
		os.Exit(1)
	}

	var content string
	if args[0] == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: failed to read stdin: %v\n", err)
			os.Exit(1)
		}
		content = string(data)
	} else {
		content = args[0]
	}
	sessionID := ""
	project := ""
	dir := ""

	for i := 1; i < len(args)-1; i++ {
		switch args[i] {
		case "--session":
			sessionID = args[i+1]
			i++
		case "--project":
			project = args[i+1]
			i++
		case "--dir":
			dir = args[i+1]
			i++
		}
	}

	if project == "" {
		fmt.Fprintln(os.Stderr, "mnemo: --project is required")
		os.Exit(1)
	}
	if dir == "" {
		fmt.Fprintln(os.Stderr, "mnemo: --dir is required")
		os.Exit(1)
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "mnemo: --session is required")
		os.Exit(1)
	}

	provenance := store.CLIProvenance(store.ToolMnemoCapture)
	if err := s.EnsureSession(sessionID, project, dir); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: could not ensure session: %v\n", err)
		os.Exit(1)
	}

	result, err := s.PassiveCapture(store.PassiveCaptureParams{
		SessionID:  sessionID,
		Content:    content,
		Project:    project,
		Source:     "subagent-stop",
		Provenance: provenance,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: capture failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Captured: extracted=%d saved=%d duplicates=%d\n",
		result.Extracted, result.Saved, result.Duplicates)
}
