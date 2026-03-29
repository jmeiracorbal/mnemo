package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	mcpserver "github.com/jmeiracorbal/mnemo/internal/mcp"
	"github.com/jmeiracorbal/mnemo/internal/plugin/claudecode"
	"github.com/jmeiracorbal/mnemo/internal/plugin/cursor"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := store.DefaultConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: config error: %v\n", err)
		os.Exit(1)
	}
	s, err := store.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	switch os.Args[1] {
	case "mcp":
		runMCP(s)
	case "save":
		runSave(s)
	case "search":
		runSearch(s)
	case "context":
		runContext(s)
	case "session":
		runSession(s)
	case "stats":
		runStats(s)
	case "export":
		runExport(s)
	case "import":
		runImport(s)
	case "capture":
		runCapture(s)
	case "setup":
		runSetup()
	case "--version", "version":
		fmt.Printf("mnemo %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "mnemo: unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// ─── mcp ─────────────────────────────────────────────────────────────────────

func runMCP(s *store.Store) {
	tools := ""
	for _, arg := range os.Args[2:] {
		if len(arg) > 8 && arg[:8] == "--tools=" {
			tools = arg[8:]
		}
	}

	allowlist := mcpserver.ResolveTools(tools)
	srv := mcpserver.NewServerWithTools(s, allowlist)

	if err := server.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: mcp server error: %v\n", err)
		os.Exit(1)
	}
}

// ─── save ────────────────────────────────────────────────────────────────────

func runSave(s *store.Store) {
	args := os.Args[2:]
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mnemo save <title> <content> [--type TYPE] [--project PROJECT] [--scope SCOPE] [--topic TOPIC_KEY]")
		os.Exit(1)
	}

	title := args[0]
	content := args[1]
	typ := "manual"
	project := ""
	scope := ""
	topicKey := ""

	for i := 2; i < len(args)-1; i++ {
		switch args[i] {
		case "--type":
			typ = args[i+1]
			i++
		case "--project":
			project = args[i+1]
			i++
		case "--scope":
			scope = args[i+1]
			i++
		case "--topic":
			topicKey = args[i+1]
			i++
		}
	}

	sessionID := "manual-save"
	if project != "" {
		sessionID = "manual-save-" + project
	}
	s.CreateSession(sessionID, project, "")

	id, err := s.AddObservation(store.AddObservationParams{
		SessionID: sessionID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Project:   project,
		Scope:     scope,
		TopicKey:  topicKey,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: save failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Memory saved: #%d %q (%s)\n", id, title, typ)
}

// ─── search ──────────────────────────────────────────────────────────────────

func runSearch(s *store.Store) {
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

	results, err := s.Search(query, opts)
	if err != nil {
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

// ─── context ─────────────────────────────────────────────────────────────────

func runContext(s *store.Store) {
	project := ""
	if len(os.Args) > 2 {
		project = os.Args[2]
	}

	ctx, err := s.FormatContext(project, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: context failed: %v\n", err)
		os.Exit(1)
	}

	if ctx == "" {
		fmt.Println("No previous session memories found.")
		return
	}

	fmt.Println(ctx)
}

// ─── session ─────────────────────────────────────────────────────────────────

func runSession(s *store.Store) {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: mnemo session start <id> [--project PROJECT] [--dir DIR]")
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

	default:
		fmt.Fprintf(os.Stderr, "mnemo: unknown session subcommand %q\n", subcmd)
		os.Exit(1)
	}
}

// ─── stats ───────────────────────────────────────────────────────────────────

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

// ─── export ──────────────────────────────────────────────────────────────────

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

// ─── import ──────────────────────────────────────────────────────────────────

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

// ─── capture ─────────────────────────────────────────────────────────────────

func runCapture(s *store.Store) {
	args := os.Args[2:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mnemo capture <content> [--session SESSION_ID] [--project PROJECT]")
		os.Exit(1)
	}

	content := args[0]
	sessionID := ""
	project := ""

	for i := 1; i < len(args)-1; i++ {
		switch args[i] {
		case "--session":
			sessionID = args[i+1]
			i++
		case "--project":
			project = args[i+1]
			i++
		}
	}

	if sessionID == "" {
		sessionID = "manual-save"
		if project != "" {
			sessionID = "manual-save-" + project
		}
	}
	s.CreateSession(sessionID, project, "")

	result, err := s.PassiveCapture(store.PassiveCaptureParams{
		SessionID: sessionID,
		Content:   content,
		Project:   project,
		Source:    "subagent-stop",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: capture failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Captured: extracted=%d saved=%d duplicates=%d\n",
		result.Extracted, result.Saved, result.Duplicates)
}

// ─── setup ───────────────────────────────────────────────────────────────────

func runSetup() {
	dryRun := false
	forCursor := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--cursor":
			forCursor = true
		}
	}

	var err error
	if forCursor {
		err = (cursor.Installer{}).Install(dryRun)
	} else {
		err = (claudecode.Installer{}).Install(dryRun)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: setup failed: %v\n", err)
		os.Exit(1)
	}
}

// ─── usage ───────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Fprintf(os.Stderr, `mnemo %s — persistent memory for AI coding agents

Usage:
  mnemo mcp [--tools=PROFILE]          Start MCP server (stdio)
  mnemo save <title> <content>         Save a memory
  mnemo search <query>                 Search memories
  mnemo context [project]              Show context from previous sessions
  mnemo session start <id>             Register session start
  mnemo session end <id>               Mark session completed
  mnemo stats                          Show memory statistics
  mnemo export [file]                  Export all memories to JSON
  mnemo import <file.json>             Import memories from JSON
  mnemo capture <content>              Extract learnings from text (passive capture)
  mnemo setup [--dry-run]              Install hooks and configure Claude Code
  mnemo setup --cursor [--dry-run]     Install hooks and configure Cursor
  mnemo version                        Show version

Tool profiles for mcp:
  --tools=agent    11 tools for AI agents (default when using plugin)
  --tools=admin    3 tools for curation (delete, stats, timeline)
  --tools=all      All tools

Storage: ~/.mnemo/memory.db
`, version)
}
