package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo/client"
	"github.com/jmeiracorbal/mnemo/protocol"
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
	input := protocol.ObservationSearchInput{Query: args[0], Limit: 10}
	for i := 1; i < len(args)-1; i++ {
		switch args[i] {
		case "--type": input.Type = args[i+1]; i++
		case "--project": input.ProjectID = args[i+1]; i++
		case "--scope": input.Scope = args[i+1]; i++
		case "--limit":
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "mnemo search: invalid limit: %v\n", err)
				os.Exit(1)
			}
			input.Limit = n
			i++
		}
	}
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo search: %v\n", err); os.Exit(1) }
	results, err := api.Search(context.Background(), input)
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo search: %v\n", err); os.Exit(1) }
	if len(results) == 0 {
		fmt.Printf("No memories found for: %q\n", input.Query)
		return
	}
	fmt.Printf("Found %d memories:\n\n", len(results))
	for i, match := range results {
		o := match.Observation
		content := o.Content
		if len(content) > 300 {
			content = content[:300] + "... [truncated — use mnemo get <ref> for full content]"
		}
		fmt.Printf("[%d] (%s) — %s\n    %s\n    ref: %s\n    %s | project: %s | scope: %s\n\n",
			i+1, o.Type, o.Title, content, o.Ref.String(),
			o.CreatedAt, o.Ref.ProjectID, o.Scope)
	}
}

func runGet() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo get <mnemo://project-id/observations/sync-id>")
		os.Exit(1)
	}
	ref, err := protocol.ParseResourceRef(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo get: %v\n", err)
		os.Exit(1)
	}
	config, err := store.DefaultConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo get: config: %v\n", err)
		os.Exit(1)
	}
	api, err := client.New(config.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo get: controller unavailable: %v\n", err)
		os.Exit(1)
	}
	observation, err := api.Get(context.Background(), ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo get: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s [%s] %s\n%s\nProject: %s\nSession: %s\nScope: %s\nCreated: %s\n",
		observation.Ref.String(), observation.Type, observation.Title,
		observation.Content, observation.Ref.ProjectID,
		observation.SessionID, observation.Scope, observation.CreatedAt)
}

// runContext routes through the controller. It deliberately does not open a
// Store: hook processes must never touch SQLite, even to initialize it.
func runContext() {
	project := ""
	if len(os.Args) > 2 { project = os.Args[2] }
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo context: %v\n", err); os.Exit(1) }
	output, err := api.Context(context.Background(), protocol.ContextBuildInput{ProjectID: project})
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo context: %v\n", err); os.Exit(1) }
	if output == "" { fmt.Println("No previous session memories found."); return }
	fmt.Println(output)
}

func loadMnemoClient() (*client.Client, error) {
	cfg, err := store.DefaultConfig()
	if err != nil { return nil, err }
	return client.New(cfg.DataDir)
}

// runSession handles session subcommands. Lifecycle operations (start/compact/end)
// are retired: session lifecycle is owned exclusively by the controller. Read-only
// queries (exists/obs-count/project-obs-count) route through controller RPC.
func runSession() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: mnemo session exists|obs-count|project-obs-count <session-id>")
		os.Exit(1)
	}
	subcmd, id := os.Args[2], os.Args[3]
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo session: %v\n", err); os.Exit(1) }
	ctx := context.Background()
	switch subcmd {
	case "exists":
		exists, err := api.SessionExists(ctx, id)
		if err != nil { fmt.Fprintf(os.Stderr, "mnemo session exists: %v\n", err); os.Exit(1) }
		fmt.Println(exists)
	case "obs-count":
		n, err := api.SessionObservationCount(ctx, id)
		if err != nil { fmt.Fprintf(os.Stderr, "mnemo session obs-count: %v\n", err); os.Exit(1) }
		fmt.Println(n)
	case "project-obs-count":
		n, err := api.SessionProjectObservationCount(ctx, id)
		if err != nil { fmt.Fprintf(os.Stderr, "mnemo session project-obs-count: %v\n", err); os.Exit(1) }
		fmt.Println(n)
	default:
		fmt.Fprintf(os.Stderr, "mnemo: unknown session subcommand %q\n", subcmd)
		os.Exit(1)
	}
}

func runStats() {
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo stats: %v\n", err); os.Exit(1) }
	stats, err := api.Stats(context.Background())
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo stats: %v\n", err); os.Exit(1) }
	fmt.Printf("Memory stats:\n")
	fmt.Printf("  Sessions:     %d\n", stats.TotalSessions)
	fmt.Printf("  Observations: %d\n", stats.TotalObservations)
	fmt.Printf("  Prompts:      %d\n", stats.TotalPrompts)
	if len(stats.Projects) > 0 {
		fmt.Printf("  Projects:     %v\n", stats.Projects)
	}
}

func runExport() {
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo export: %v\n", err); os.Exit(1) }
	archive, err := api.Export(context.Background())
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo export: %v\n", err); os.Exit(1) }
	out, err := json.MarshalIndent(archive, "", "  ")
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo export: json: %v\n", err); os.Exit(1) }
	if len(os.Args) > 2 {
		if err := os.WriteFile(os.Args[2], out, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo export: write: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported to %s\n", os.Args[2])
		return
	}
	fmt.Println(string(out))
}

func runImport() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo import <file.json>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[2])
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo import: read: %v\n", err); os.Exit(1) }
	var archive protocol.ArchiveExportOutput
	if err := json.Unmarshal(data, &archive); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo import: invalid archive: %v\n", err)
		os.Exit(1)
	}
	api, err := loadMnemoClient()
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo import: %v\n", err); os.Exit(1) }
	result, err := api.Import(context.Background(), uuid.NewString(), protocol.ArchiveImportInput{Archive: archive})
	if err != nil { fmt.Fprintf(os.Stderr, "mnemo import: %v\n", err); os.Exit(1) }
	fmt.Printf("Import complete: %d sessions, %d observations\n", result.SessionsImported, result.ObservationsImported)
}

func runCapture() {
	fmt.Fprintln(os.Stderr, "mnemo capture: this command is deprecated.")
	fmt.Fprintln(os.Stderr, "  Session identity is now owned by the controller runtime.")
	fmt.Fprintln(os.Stderr, "  Use the MCP tool mem_capture_passive to capture learnings through an active agent session.")
	os.Exit(1)
}
