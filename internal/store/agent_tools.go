package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmeiracorbal/mnemo/adapters"
	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

// AgentToolRequest is the controller contract used by a native agent adapter.
// Agent extensions submit their platform-native ID, never mnemo's session ID
// nor a precomputed execution key.
type AgentToolRequest struct {
	Agent     adapters.Agent `json:"agent"`
	NativeID  string         `json:"native_id"`
	Project   string         `json:"project"`
	Directory string         `json:"directory"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

// AgentToolResult is deliberately transport-neutral so native extensions can
// render the same result as an MCP tool without learning SQLite details.
type AgentToolResult struct {
	Text string `json:"text"`
}

// ExecuteAgentTool is the controller-only entry point for native adapters.
// It intentionally covers the agent MCP profile; diagnostics remain CLI tools.
func (s *Store) ExecuteAgentTool(input AgentToolRequest) (AgentToolResult, error) {
	identity, err := adapters.NewIdentity(input.Agent, input.Project, input.NativeID)
	if err != nil {
		return AgentToolResult{}, fmt.Errorf("derive agent execution identity: %w", err)
	}
	if strings.TrimSpace(input.Tool) == "" {
		return AgentToolResult{}, fmt.Errorf("agent tool is required")
	}
	if input.Arguments == nil {
		input.Arguments = map[string]any{}
	}

	args := input.Arguments
	// These calls have no session side effect. They still require an explicit,
	// valid adapter identity above so all native calls use one contract.
	switch input.Tool {
	case "mem_search":
		value, err := s.Search(stringArg(args, "query"), SearchOptions{Type: stringArg(args, "type"), Project: stringArg(args, "project"), Scope: stringArg(args, "scope"), TopicKey: stringArg(args, "topic_key"), Tags: splitTags(stringArg(args, "tags")), PreferTags: splitTags(stringArg(args, "prefer_tags")), Limit: intArg(args, "limit", 10)})
		return agentToolJSON(value, err)
	case "mem_suggest_topic_key":
		title, content := stringArg(args, "title"), stringArg(args, "content")
		if strings.TrimSpace(title) == "" && strings.TrimSpace(content) == "" {
			return AgentToolResult{}, fmt.Errorf("provide title or content to suggest a topic_key")
		}
		return agentToolJSON(map[string]any{"topic_key": SuggestTopicKey(stringArg(args, "type"), title, content), "suggested_tags": SuggestTags(stringArg(args, "type"), title, content)}, nil)
	case "mem_context":
		value, err := s.FormatContextOpts(stringArg(args, "project"), stringArg(args, "scope"), ContextOptions{Tags: splitTags(stringArg(args, "tags")), PreferTags: splitTags(stringArg(args, "prefer_tags")), TopicKey: stringArg(args, "topic_key")})
		return agentToolJSON(value, err)
	case "mem_get_observation":
		id := int64(intArg(args, "id", 0))
		if id == 0 {
			return AgentToolResult{}, fmt.Errorf("id is required")
		}
		value, err := s.GetObservation(id)
		return agentToolJSON(value, err)
	case "mem_update":
		return s.executeAgentUpdate(args)
	case "mem_list_tags":
		value, err := s.ListTags(stringArg(args, "project"))
		return agentToolJSON(value, err)
	case "mem_merge_tags":
		from, to := stringArg(args, "from"), stringArg(args, "to")
		if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
			return AgentToolResult{}, fmt.Errorf("both 'from' and 'to' are required")
		}
		observations, sessions, err := s.MergeTags(from, to)
		return agentToolJSON(map[string]int{"observations": observations, "sessions": sessions}, err)
	case "mem_tag_stats":
		return s.executeAgentTagStats(args)
	case "mem_related_tags":
		return s.executeAgentRelatedTags(args)
	}

	// Memory-writing tools have a mandatory canonical project and workspace and
	// resolve exactly one controller-owned execution session.
	if strings.TrimSpace(input.Project) == "" {
		return AgentToolResult{}, fmt.Errorf("project is required")
	}
	if strings.TrimSpace(input.Directory) == "" {
		return AgentToolResult{}, fmt.Errorf("directory is required")
	}
	sessionID, err := s.ensureExecutionSession(identity, input.Directory)
	if err != nil {
		return AgentToolResult{}, err
	}
	provenance := AgentAdapterProvenance(string(input.Agent), input.Tool)

	switch input.Tool {
	case "mem_save":
		typ := stringArg(args, "type")
		if typ == "" {
			typ = "manual"
		}
		title, content := stringArg(args, "title"), stringArg(args, "content")
		if _, err := s.AddObservation(AddObservationParams{SessionID: sessionID, Type: typ, Title: title, Content: content, Project: input.Project, Scope: stringArg(args, "scope"), TopicKey: stringArg(args, "topic_key"), Tags: splitTags(stringArg(args, "tags")), Provenance: provenance}); err != nil {
			return AgentToolResult{}, err
		}
		return AgentToolResult{Text: fmt.Sprintf("Memory saved: %q (%s)", title, typ)}, nil
	case "mem_save_prompt":
		content := stringArg(args, "content")
		if _, err := s.AddPrompt(AddPromptParams{SessionID: sessionID, Content: content, Project: input.Project, Provenance: provenance}); err != nil {
			return AgentToolResult{}, err
		}
		return AgentToolResult{Text: fmt.Sprintf("Prompt saved: %q", truncateAgentTool(content, 80))}, nil
	case "mem_session_summary":
		if _, err := s.AddObservation(AddObservationParams{SessionID: sessionID, Type: "session_summary", Title: fmt.Sprintf("Session summary: %s", input.Project), Content: stringArg(args, "content"), Project: input.Project, Provenance: provenance}); err != nil {
			return AgentToolResult{}, err
		}
		return AgentToolResult{Text: fmt.Sprintf("Session summary saved for project %q", input.Project)}, nil
	case "mem_capture_passive":
		content := stringArg(args, "content")
		if content == "" {
			return AgentToolResult{}, fmt.Errorf("content is required — include text with a '## Key Learnings:' section")
		}
		source := stringArg(args, "source")
		if source == "" {
			source = "pi-extension"
		}
		value, err := s.PassiveCapture(PassiveCaptureParams{SessionID: sessionID, Content: content, Project: input.Project, Source: source, Provenance: provenance})
		return agentToolJSON(value, err)
	default:
		return AgentToolResult{}, fmt.Errorf("unsupported native agent tool %q", input.Tool)
	}
}

func (s *Store) ensureExecutionSession(identity adapters.Identity, directory string) (string, error) {
	var sessionID string
	err := s.withTx(func(tx *sql.Tx) error {
		var err error
		sessionID, err = s.ensureExecutionSessionTx(tx, identity, directory)
		return err
	})
	return sessionID, err
}

func (s *Store) ensureExecutionSessionTx(tx *sql.Tx, identity adapters.Identity, directory string) (string, error) {
	q := s.q.WithTx(tx)
	id := "execution-" + identity.Execution
	endedAt, err := q.GetExecutionSessionEndedAt(context.Background(), dbgen.GetExecutionSessionEndedAtParams{ID: id, Project: identity.Project})
	if err == sql.ErrNoRows {
		if err := s.ensureProjectTx(tx, identity.Project); err != nil {
			return "", err
		}
		if err := s.createSessionTx(tx, id, identity.Project, directory, ProvenanceInput{}); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else if endedAt.Valid {
		return "", fmt.Errorf("execution is already closed")
	}
	if err := q.InsertExecutionSessionIfMissing(context.Background(), dbgen.InsertExecutionSessionIfMissingParams{
		Project: identity.Project, ExecutionKey: identity.Execution, SessionID: id,
	}); err != nil {
		return "", err
	}
	if _, err := q.GetSessionPayload(context.Background(), id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) executeAgentUpdate(args map[string]any) (AgentToolResult, error) {
	id := int64(intArg(args, "id", 0))
	if id == 0 {
		return AgentToolResult{}, fmt.Errorf("id is required")
	}
	update := UpdateObservationParams{}
	if v, ok := optionalString(args, "title"); ok {
		update.Title = &v
	}
	if v, ok := optionalString(args, "content"); ok {
		update.Content = &v
	}
	if v, ok := optionalString(args, "type"); ok {
		update.Type = &v
	}
	if v, ok := optionalString(args, "project"); ok {
		update.Project = &v
	}
	if v, ok := optionalString(args, "scope"); ok {
		update.Scope = &v
	}
	if v, ok := optionalString(args, "topic_key"); ok {
		update.TopicKey = &v
	}
	if v, ok := optionalString(args, "tags"); ok {
		tags := splitTags(v)
		update.Tags = &tags
	}
	if update.Title == nil && update.Content == nil && update.Type == nil && update.Project == nil && update.Scope == nil && update.TopicKey == nil && update.Tags == nil {
		return AgentToolResult{}, fmt.Errorf("provide at least one field to update")
	}
	value, err := s.UpdateObservation(id, update)
	return agentToolJSON(value, err)
}

func (s *Store) executeAgentTagStats(args map[string]any) (AgentToolResult, error) {
	sortBy := stringArg(args, "sort_by")
	if sortBy != "" && sortBy != "freq" && sortBy != "stale" && sortBy != "alpha" {
		return AgentToolResult{}, fmt.Errorf("invalid sort_by %q", sortBy)
	}
	opts := TagStatsOptions{MinCount: intArg(args, "min_count", 0), MaxCount: intArg(args, "max_count", 0), Limit: intArg(args, "limit", 20), SortBy: sortBy}
	if opts.MinCount < 0 || opts.MaxCount < 0 || opts.Limit < 0 {
		return AgentToolResult{}, fmt.Errorf("tag counts and limit must be >= 0")
	}
	if raw := stringArg(args, "unused_since"); raw != "" {
		value, err := parseAgentToolTime(raw)
		if err != nil {
			return AgentToolResult{}, err
		}
		opts.UnusedSince = value
	}
	value, err := s.TagStats(stringArg(args, "project"), opts)
	return agentToolJSON(value, err)
}

func (s *Store) executeAgentRelatedTags(args map[string]any) (AgentToolResult, error) {
	tag := stringArg(args, "tag")
	if tag == "" {
		return AgentToolResult{}, fmt.Errorf("tag is required")
	}
	includeObservations, observationsProvided := boolArg(args, "include_observations")
	includeSessions, sessionsProvided := boolArg(args, "include_sessions")
	if !observationsProvided {
		includeObservations = true
	}
	if !sessionsProvided {
		includeSessions = true
	}
	opts := RelatedTagsOptions{Limit: intArg(args, "limit", 20), MinCooccurrence: intArg(args, "min_cooccurrence", 0), IncludeObservations: includeObservations, IncludeSessions: includeSessions}
	if opts.Limit < 0 || opts.MinCooccurrence < 0 {
		return AgentToolResult{}, fmt.Errorf("limit and min_cooccurrence must be >= 0")
	}
	if !includeObservations && !includeSessions {
		return AgentToolResult{}, fmt.Errorf("at least one include flag must be true")
	}
	if raw := stringArg(args, "since"); raw != "" {
		value, err := parseAgentToolTime(raw)
		if err != nil {
			return AgentToolResult{}, err
		}
		opts.Since = value
	}
	value, err := s.RelatedTags(stringArg(args, "project"), tag, opts)
	return agentToolJSON(value, err)
}

func agentToolJSON(value any, err error) (AgentToolResult, error) {
	if err != nil {
		return AgentToolResult{}, err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return AgentToolResult{}, err
	}
	return AgentToolResult{Text: string(data)}, nil
}
func stringArg(args map[string]any, key string) string { value, _ := args[key].(string); return value }
func optionalString(args map[string]any, key string) (string, bool) {
	value, ok := args[key].(string)
	return value, ok
}
func intArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key].(float64)
	if !ok {
		return fallback
	}
	return int(value)
}
func boolArg(args map[string]any, key string) (bool, bool) {
	value, ok := args[key].(bool)
	return value, ok
}
func splitTags(raw string) []string {
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		if tag := strings.TrimSpace(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
func parseAgentToolTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: use ISO8601", value)
	}
	return parsed, nil
}
func truncateAgentTool(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
