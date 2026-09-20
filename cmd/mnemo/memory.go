package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/jmeiracorbal/mnemo-events"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

// runSave is retired: session lifecycle is now owned by the controller.
// Use the MCP tool mem_save instead.
func runSave() {
	fmt.Fprintln(os.Stderr, "mnemo save: this command is deprecated.")
	fmt.Fprintln(os.Stderr, "  Session identity is now owned by the controller runtime.")
	fmt.Fprintln(os.Stderr, "  Use the MCP tool mem_save to save memories through an active agent session.")
	os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
		os.Exit(1)
	}

	var results []store.SearchResult
	input := struct {
		Query   string              `json:"query"`
		Options store.SearchOptions `json:"options"`
	}{query, opts}
	if err := events.Call(context.Background(), cfg, "search", input, &results); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: search failed: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
		os.Exit(1)
	}

	var output string
	input := struct {
		Project string               `json:"project"`
		Scope   string               `json:"scope"`
		Options store.ContextOptions `json:"options"`
	}{project, "", store.ContextOptions{}}
	if err := events.Call(context.Background(), cfg, "format_context", input, &output); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: context failed: %v\n", err)
		os.Exit(1)
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

// runSession handles session subcommands. Lifecycle operations (start/compact/end)
// are retired: session lifecycle is owned exclusively by the controller. Read-only
// queries (exists/obs-count/project-obs-count) route through controller RPC.
func runSession() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: mnemo session exists <id>")
		fmt.Fprintln(os.Stderr, "       mnemo session obs-count <id>")
		fmt.Fprintln(os.Stderr, "       mnemo session project-obs-count <id>")
		fmt.Fprintln(os.Stderr, "note: start/compact/end are retired; session lifecycle is owned by the controller")
		os.Exit(1)
	}

	subcmd := os.Args[2]
	id := os.Args[3]

	switch subcmd {
	case "start", "compact", "end":
		fmt.Fprintf(os.Stderr, "mnemo session %s: this subcommand is deprecated.\n", subcmd)
		fmt.Fprintln(os.Stderr, "  Session lifecycle is owned by the controller runtime via durable events.")
		os.Exit(1)

	case "exists":
		cfg, err := loadEventsConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
			os.Exit(1)
		}
		var exists bool
		input := struct {
			ID string `json:"id"`
		}{id}
		if err := events.Call(context.Background(), cfg, "session_exists", input, &exists); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: session exists failed: %v\n", err)
			os.Exit(1)
		}
		if !exists {
			fmt.Println("false")
			os.Exit(1)
		}
		fmt.Println("true")

	case "obs-count":
		cfg, err := loadEventsConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
			os.Exit(1)
		}
		var n int
		input := struct {
			ID string `json:"id"`
		}{id}
		if err := events.Call(context.Background(), cfg, "session_obs_count", input, &n); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: obs-count failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(n)

	case "project-obs-count":
		cfg, err := loadEventsConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
			os.Exit(1)
		}
		var n int
		input := struct {
			ID string `json:"id"`
		}{id}
		if err := events.Call(context.Background(), cfg, "session_project_obs_count", input, &n); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: project-obs-count failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(n)

	default:
		fmt.Fprintf(os.Stderr, "mnemo: unknown session subcommand %q\n", subcmd)
		os.Exit(1)
	}
}

func runStats() {
	cfg, err := loadEventsConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
		os.Exit(1)
	}

	var stats store.Stats
	if err := events.Call(context.Background(), cfg, "stats", struct{}{}, &stats); err != nil {
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

func runExport() {
	cfg, err := loadEventsConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
		os.Exit(1)
	}

	var data store.ExportData
	if err := events.Call(context.Background(), cfg, "export", struct{}{}, &data); err != nil {
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

func runImport() {
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

	cfg, err := loadEventsConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: controller unavailable: %v\n", err)
		os.Exit(1)
	}

	var result store.ImportResult
	if err := events.Call(context.Background(), cfg, "import", payload, &result); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: import failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Import complete: %d sessions, %d observations\n", result.SessionsImported, result.ObservationsImported)
}

// runCapture is retired: session lifecycle is now owned by the controller.
// Use the MCP tool mem_capture_passive instead.
func runCapture() {
	fmt.Fprintln(os.Stderr, "mnemo capture: this command is deprecated.")
	fmt.Fprintln(os.Stderr, "  Session identity is now owned by the controller runtime.")
	fmt.Fprintln(os.Stderr, "  Use the MCP tool mem_capture_passive to capture learnings through an active agent session.")
	os.Exit(1)
}
