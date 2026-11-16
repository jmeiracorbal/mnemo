package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo/v2/client"
	"github.com/jmeiracorbal/mnemo/v2/internal/adapters/agents"
	"github.com/jmeiracorbal/mnemo/v2/internal/store"
	"github.com/jmeiracorbal/mnemo/v2/protocol"
)

// invokeNativeMnemoTool is the native agent presentation adapter. It invokes
// only Mnemo's public domain operations, never controller action-string RPCs.
func invokeNativeMnemoTool(ctx context.Context, api *client.Client,
	project, directory, agent, nativeID, tool string, args map[string]any,
) (string, error) {
	if _, err := agents.NewIdentity(agents.Agent(agent), project, nativeID); err != nil {
		return "", err
	}
	if args == nil { args = map[string]any{} }
	jsonText := func(value any) (string, error) {
		data, err := json.MarshalIndent(value, "", "  ")
		return string(data), err
	}
	stringArg := func(key string) string {
		value, _ := args[key].(string)
		return value
	}
	limitArg := func(key string, fallback int) int {
		value, ok := args[key].(float64)
		if !ok { return fallback }
		return int(value)
	}
	tags := func(key string) []string {
		var out []string
		for _, value := range strings.Split(stringArg(key), ",") {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	// These tools only read Mnemo data; no execution session is created.
	switch tool {
	case "mem_search":
		value, err := api.Search(ctx, protocol.ObservationSearchInput{
			Query: stringArg("query"), ProjectID: stringArg("project"),
			Type: stringArg("type"), Scope: stringArg("scope"),
			TopicKey: stringArg("topic_key"), Tags: tags("tags"),
			PreferTags: tags("prefer_tags"), Limit: limitArg("limit", 10),
		})
		if err != nil { return "", err }
		return jsonText(value)
	case "mem_suggest_topic_key":
		title, content := stringArg("title"), stringArg("content")
		if title == "" && content == "" { return "", fmt.Errorf("title or content is required") }
		return jsonText(map[string]any{
			"topic_key": store.SuggestTopicKey(stringArg("type"), title, content),
			"suggested_tags": store.SuggestTags(stringArg("type"), title, content),
		})
	case "mem_context":
		value, err := api.Context(ctx, protocol.ContextBuildInput{
			ProjectID: stringArg("project"), Scope: stringArg("scope"),
			TopicKey: stringArg("topic_key"), Tags: tags("tags"), PreferTags: tags("prefer_tags"),
		})
		if err != nil { return "", err }
		return jsonText(value)
	case "mem_get_observation":
		ref, err := protocol.ParseResourceRef(stringArg("ref"))
		if err != nil { return "", err }
		value, err := api.Get(ctx, ref)
		if err != nil { return "", err }
		return jsonText(value)
	case "mem_list_tags":
		value, err := api.ListTags(ctx, stringArg("project"))
		if err != nil { return "", err }
		return jsonText(value)
	case "mem_tag_stats":
		unused, err := nativeToolDate(stringArg("unused_since"))
		if err != nil { return "", err }
		value, err := api.TagStats(ctx, protocol.TagStatsInput{
			ProjectID: stringArg("project"),
			Options: protocol.TagStatsOptions{
				MinCount: limitArg("min_count", 0), MaxCount: limitArg("max_count", 0),
				UnusedSince: unused, Limit: limitArg("limit", 20),
				SortBy: stringArg("sort_by"),
			},
		})
		if err != nil { return "", err }
		return jsonText(value)
	case "mem_related_tags":
		since, err := nativeToolDate(stringArg("since"))
		if err != nil { return "", err }
		includeObservations, _ := args["include_observations"].(bool)
		includeSessions, _ := args["include_sessions"].(bool)
		if _, ok := args["include_observations"]; !ok { includeObservations = true }
		if _, ok := args["include_sessions"]; !ok { includeSessions = true }
		value, err := api.RelatedTags(ctx, protocol.TagRelatedInput{
			ProjectID: stringArg("project"), Tag: stringArg("tag"),
			Options: protocol.TagRelatedOptions{
				Limit: limitArg("limit", 20), Since: since,
				MinCooccurrence: limitArg("min_cooccurrence", 0),
				IncludeObservations: includeObservations, IncludeSessions: includeSessions,
			},
		})
		if err != nil { return "", err }
		return jsonText(value)
	case "mem_update":
		ref, err := protocol.ParseResourceRef(stringArg("ref"))
		if err != nil { return "", err }
		changes := protocol.ObservationChanges{}
		if value, ok := args["title"].(string); ok { changes.Title = &value }
		if value, ok := args["content"].(string); ok { changes.Content = &value }
		if value, ok := args["type"].(string); ok { changes.Type = &value }
		if value, ok := args["scope"].(string); ok { changes.Scope = &value }
		if value, ok := args["topic_key"].(string); ok { changes.TopicKey = &value }
		if _, ok := args["tags"]; ok {
			parsed := tags("tags")
			changes.Tags = &parsed
		}
		updated, err := api.Update(ctx, uuid.NewString(), protocol.ObservationUpdateInput{Ref: ref, Changes: changes})
		if err != nil { return "", err }
		return jsonText(updated)
	case "mem_merge_tags":
		merged, err := api.MergeTags(ctx, uuid.NewString(), protocol.TagMergeInput{
			From: stringArg("from"), To: stringArg("to"),
		})
		if err != nil { return "", err }
		return jsonText(merged)

	}

	// Mutating tools resolve the real native agent execution session, preserving
	// its identity rather than allocating an unrelated MCP session.
	sessionID, err := api.ResolveSession(ctx, uuid.NewString(), protocol.SessionResolveInput{
		ProjectID: project, Directory: directory, AgentID: agent, NativeID: nativeID,
	})
	if err != nil { return "", err }
	provenance := protocol.Provenance{AgentID: agent, SourceKindID: "agent_adapter", ToolID: tool}
	switch tool {
	case "mem_save":
		kind := stringArg("type")
		if kind == "" { kind = "manual" }
		title, content := stringArg("title"), stringArg("content")
		ref, err := api.Create(ctx, uuid.NewString(), protocol.ObservationCreateInput{
			ProjectID: project, SessionID: sessionID, Type: kind,
			Title: title, Content: content, Scope: stringArg("scope"),
			TopicKey: stringArg("topic_key"), Tags: tags("tags"), Provenance: provenance,
		})
		if err != nil { return "", err }
		return fmt.Sprintf("Memory saved: %q (%s)\nReference: %s", title, kind, ref.String()), nil
	case "mem_save_prompt":
		content := stringArg("content")
		if err := api.CreatePrompt(ctx, uuid.NewString(), protocol.PromptCreateInput{
			ProjectID: project, SessionID: sessionID, Content: content, Provenance: provenance,
		}); err != nil { return "", err }
		return fmt.Sprintf("Prompt saved: %q", content), nil
	case "mem_session_summary":
		content := stringArg("content")
		_, err := api.Create(ctx, uuid.NewString(), protocol.ObservationCreateInput{
			ProjectID: project, SessionID: sessionID, Type: "session_summary",
			Title: fmt.Sprintf("Session summary: %s", project),
			Content: content, Provenance: provenance,
		})
		if err != nil { return "", err }
		return fmt.Sprintf("Session summary saved for project %q", project), nil
	case "mem_capture_passive":
		source := stringArg("source")
		if source == "" { source = "pi-extension" }
		value, err := api.CapturePassive(ctx, uuid.NewString(), protocol.CapturePassiveInput{
			ProjectID: project, SessionID: sessionID, Content: stringArg("content"),
			Source: source, Provenance: provenance,
		})
		if err != nil { return "", err }
		return jsonText(value)
	default:
		return "", fmt.Errorf("unknown native Mnemo tool %q", tool)
	}
}

func nativeToolDate(value string) (time.Time, error) {
	if value == "" { return time.Time{}, nil }
	if t, err := time.Parse(time.RFC3339, value); err == nil { return t, nil }
	t, err := time.Parse("2006-01-02", value)
	if err != nil { return time.Time{}, fmt.Errorf("invalid date %q", value) }
	return t, nil
}
