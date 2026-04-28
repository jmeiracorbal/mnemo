package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	mcpserver "github.com/jmeiracorbal/mnemo/internal/mcp"
	"github.com/jmeiracorbal/mnemo/internal/jsonmerge"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Commands that don't need the store.
	switch os.Args[1] {
	case "json":
		runJSON()
		return
	case "json-merge":
		runJSONMerge()
		return
	case "extract-transcript":
		runExtractTranscript()
		return
	case "init":
		runInit()
		return
	case "--version", "version":
		fmt.Printf("mnemo %s\n", version)
		return
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
	if err := s.CreateSession(sessionID, project, ""); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: warning: could not create session: %v\n", err)
	}

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
	if err := s.CreateSession(sessionID, project, ""); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: warning: could not create session: %v\n", err)
	}

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

// ─── json ────────────────────────────────────────────────────────────────────

// runJSON reads JSON from stdin and extracts a value by key path.
// Intended for use in hook scripts as a replacement for inline python3 -c.
//
// Usage: mnemo json KEY [KEY ...]
// Examples:
//
//	mnemo json session_id
//	mnemo json workspace_roots 0
//	mnemo json tool_info transcript_path
func runJSON() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo json KEY [KEY ...]")
		os.Exit(1)
	}

	var data any
	if err := json.NewDecoder(os.Stdin).Decode(&data); err != nil {
		fmt.Println("")
		return
	}

	value := data
	for _, key := range os.Args[2:] {
		switch v := value.(type) {
		case map[string]any:
			value = v[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(v) {
				value = nil
			} else {
				value = v[i]
			}
		default:
			value = nil
		}
		if value == nil {
			break
		}
	}

	switch v := value.(type) {
	case string:
		fmt.Println(v)
	case float64:
		if v == float64(int64(v)) {
			fmt.Println(int64(v))
		} else {
			fmt.Println(v)
		}
	case bool:
		fmt.Println(v)
	default:
		fmt.Println("")
	}
}

// ─── extract-transcript ───────────────────────────────────────────────────────

// runExtractTranscript reads a JSONL transcript file and prints the text
// content of all assistant messages. Intended for passive capture in hooks.
//
// Usage: mnemo extract-transcript /path/to/transcript.jsonl
func runExtractTranscript() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo extract-transcript <file.jsonl>")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[2])
	if err != nil {
		// Silent failure: transcript may not exist yet.
		return
	}
	defer f.Close()

	type block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var msg message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil || msg.Role != "assistant" {
			continue
		}

		// Try plain string content first.
		var text string
		if err := json.Unmarshal(msg.Content, &text); err == nil {
			if text != "" {
				lines = append(lines, text)
			}
			continue
		}

		// Try array of content blocks.
		var blocks []block
		if err := json.Unmarshal(msg.Content, &blocks); err == nil {
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					lines = append(lines, b.Text)
				}
			}
		}
	}

	fmt.Println(strings.Join(lines, "\n"))
}

// ─── init ────────────────────────────────────────────────────────────────────

// runInit configures mnemo for one or more agents in the current project.
func runInit() {
	agent := "claudecode"
	dir := "."

	for _, arg := range os.Args[2:] {
		switch {
		case strings.HasPrefix(arg, "--agent="):
			agent = arg[len("--agent="):]
		case strings.HasPrefix(arg, "--path="):
			dir = arg[len("--path="):]
		}
	}

	abs, err := absPath(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo init: %v\n", err)
		os.Exit(1)
	}
	root := agentinit.ProjectRoot(abs)

	agents := []string{agent}
	if agent == "all" {
		agents = []string{"claudecode", "cursor", "windsurf", "codex"}
	}

	for _, a := range agents {
		if err := initAgent(root, a); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo init: %s: %v\n", a, err)
			os.Exit(1)
		}
		fmt.Printf("mnemo init: %s configured in %s\n", a, root)
	}
}

func initAgent(root, agent string) error {
	switch agent {
	case "claudecode":
		return agentinit.InitClaudeCode(root)
	case "cursor":
		return agentinit.InitCursor(root)
	case "windsurf":
		return agentinit.InitWindsurf(root)
	case "codex":
		return agentinit.InitCodex(root)
	default:
		return fmt.Errorf("unknown agent %q — valid: claudecode | cursor | windsurf | codex | all", agent)
	}
}

func absPath(dir string) (string, error) {
	if dir == "." {
		return os.Getwd()
	}
	return dir, nil
}

// ─── json-merge ──────────────────────────────────────────────────────────────

// runJSONMerge reads a JSON patch from stdin and deep-merges it into FILE.
// Usage: mnemo json-merge <file>
func runJSONMerge() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo json-merge <file>")
		os.Exit(1)
	}
	changed, err := jsonmerge.MergeFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: json-merge: %v\n", err)
		os.Exit(1)
	}
	if changed {
		fmt.Println("updated")
	} else {
		fmt.Println("already up to date")
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
  mnemo init [--agent=AGENT]           Configure mnemo in the current project
  mnemo json KEY [KEY ...]             Extract field from JSON on stdin (used by hooks)
  mnemo json-merge <file>              Deep-merge JSON from stdin into file
  mnemo extract-transcript <file>      Extract assistant text from JSONL transcript
  mnemo version                        Show version

Agents for init:
  --agent=claudecode   AGENTS.md + CLAUDE.md symlink (default)
  --agent=cursor       .cursor/hooks.json + .cursor/rules/mnemo.mdc
  --agent=windsurf     .windsurf/hooks.json + .windsurf/rules/mnemo.md
  --agent=codex        AGENTS.md append
  --agent=all          All agents

Tool profiles for mcp:
  --tools=agent    11 tools for AI agents (default when using plugin)
  --tools=admin    3 tools for curation (delete, stats, timeline)
  --tools=all      All tools

Storage: ~/.mnemo/memory.db
`, version)
}
