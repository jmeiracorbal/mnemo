package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo/protocol"
		"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func handleSearch(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		query, _ := args["query"].(string)
		typ, _ := args["type"].(string)
		project, _ := args["project"].(string)
		scope, _ := args["scope"].(string)
		topicKey, _ := args["topic_key"].(string)
		tagsRaw, _ := args["tags"].(string)
		preferRaw, _ := args["prefer_tags"].(string)
		results, err := s.Search(ctx, protocol.ObservationSearchInput{
			Query: query, ProjectID: project, Type: typ, Scope: scope,
			TopicKey: topicKey, Tags: parseTags(tagsRaw),
			PreferTags: parseTags(preferRaw), Limit: intArg(req, "limit", 10),
		})
		if err != nil {
			return mcp.NewToolResultError("Search error: " + err.Error()), nil
		}
		if len(results) == 0 {
			if query == "" { return mcp.NewToolResultText("No memories found matching the specified filters."), nil }
			return mcp.NewToolResultText(fmt.Sprintf("No memories found for: %q", query)), nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Found %d memories:\n\n", len(results))
		truncated := false
		for i, match := range results {
			o := match.Observation
			preview := truncate(o.Content, 300)
			if len(o.Content) > 300 {
				truncated = true
				preview += " [truncated — use mem_get_observation(ref) for full content]"
			}
			fmt.Fprintf(&b, "[%d] (%s) — %s\n    %s\n    ref: %s\n    %s | project: %s | scope: %s\n\n",
				i+1, o.Type, o.Title, preview, o.Ref.String(),
				o.CreatedAt, o.Ref.ProjectID, o.Scope)
		}
		if truncated {
			b.WriteString("---\nSearch results are previews. Use mem_get_observation(ref: <RESOURCE_REF>) to read the full memory.\n")
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

func handleSave(s MemoryBackend, runtime *Runtime) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, _ := req.GetArguments()["title"].(string)
		content, _ := req.GetArguments()["content"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		project, _ := req.GetArguments()["project"].(string)
		directory, _ := req.GetArguments()["directory"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		tagsRaw, _ := req.GetArguments()["tags"].(string)
		tags := parseTags(tagsRaw)

		if err := validateMarker(project, directory); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if typ == "" {
			typ = "manual"
		}
		suggestedTopicKey := suggestTopicKey(typ, title, content)

		sessionID, err := runtime.resolveSession(s, project, directory, req)
		if err != nil {
			return mcp.NewToolResultError("No active session: " + err.Error()), nil
		}
		provenance := mcpProvenance("mem_save")

		limits, err := s.Limits(ctx)
		if err != nil {
			return mcp.NewToolResultError("Failed to read observation limits: " + err.Error()), nil
		}
		truncated := len(content) > limits.MaxObservationLength

		ref, err := s.Create(ctx, uuid.NewString(), protocol.ObservationCreateInput{
			SessionID: sessionID, ProjectID: project, Type: typ,
			Title: title, Content: content, Scope: scope, TopicKey: topicKey,
			Tags: tags,
			Provenance: provenance,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Memory saved: %q (%s)\nReference: %s", title, typ, ref.String())
		if topicKey == "" && suggestedTopicKey != "" {
			msg += fmt.Sprintf("\nSuggested topic_key: %s", suggestedTopicKey)
		}
		if len(tags) == 0 {
			if suggested := suggestTags(typ, title, content); len(suggested) > 0 {
				msg += fmt.Sprintf("\nSuggested tags: %s", strings.Join(suggested, ", "))
			}
		}
		if truncated {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", len(content), limits.MaxObservationLength)
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleSuggestTopicKey() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typ, _ := req.GetArguments()["type"].(string)
		title, _ := req.GetArguments()["title"].(string)
		content, _ := req.GetArguments()["content"].(string)

		if strings.TrimSpace(title) == "" && strings.TrimSpace(content) == "" {
			return mcp.NewToolResultError("provide title or content to suggest a topic_key"), nil
		}

		topicKey := suggestTopicKey(typ, title, content)
		if topicKey == "" {
			return mcp.NewToolResultError("could not suggest topic_key from input"), nil
		}

		msg := fmt.Sprintf("Suggested topic_key: %s", topicKey)
		if tags := suggestTags(typ, title, content); len(tags) > 0 {
			msg += fmt.Sprintf("\nSuggested tags: %s", strings.Join(tags, ", "))
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleUpdate(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, _ := req.GetArguments()["ref"].(string)
		ref, err := protocol.ParseResourceRef(raw)
		if err != nil { return mcp.NewToolResultError("Invalid Mnemo ref: " + err.Error()), nil }
		changes := protocol.ObservationChanges{}
		if v, ok := req.GetArguments()["title"].(string); ok { changes.Title = &v }
		if v, ok := req.GetArguments()["content"].(string); ok { changes.Content = &v }
		if v, ok := req.GetArguments()["type"].(string); ok { changes.Type = &v }
		if v, ok := req.GetArguments()["scope"].(string); ok { changes.Scope = &v }
		if v, ok := req.GetArguments()["topic_key"].(string); ok { changes.TopicKey = &v }
		if v, ok := req.GetArguments()["tags"].(string); ok {
			tags := parseTags(v)
			changes.Tags = &tags
		}
		if changes.Title == nil && changes.Content == nil && changes.Type == nil &&
			changes.Scope == nil && changes.TopicKey == nil && changes.Tags == nil {
			return mcp.NewToolResultError("provide at least one field to update"), nil
		}
		limits, err := s.Limits(ctx)
		if err != nil { return mcp.NewToolResultError("Failed to read observation limits: " + err.Error()), nil }
		obs, err := s.Update(ctx, uuid.NewString(), protocol.ObservationUpdateInput{Ref: ref, Changes: changes})
		if err != nil { return mcp.NewToolResultError("Failed to update memory: " + err.Error()), nil }
		msg := fmt.Sprintf("Memory updated: %s %q (%s, scope=%s)", obs.Ref.String(), obs.Title, obs.Type, obs.Scope)
		if changes.Content != nil && len(*changes.Content) > limits.MaxObservationLength {
			msg += fmt.Sprintf("\nWARNING: Content was truncated from %d to %d chars.", len(*changes.Content), limits.MaxObservationLength)
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleDelete(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, _ := req.GetArguments()["ref"].(string)
		ref, err := protocol.ParseResourceRef(raw)
		if err != nil { return mcp.NewToolResultError("Invalid Mnemo ref: " + err.Error()), nil }
		if err := s.Delete(ctx, uuid.NewString(), ref); err != nil {
			return mcp.NewToolResultError("Failed to delete memory: " + err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Memory %s soft-deleted", ref.String())), nil
	}
}

func handleSavePrompt(s MemoryBackend, runtime *Runtime) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		project, _ := req.GetArguments()["project"].(string)
		directory, _ := req.GetArguments()["directory"].(string)

		if err := validateMarker(project, directory); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sessionID, err := runtime.resolveSession(s, project, directory, req)
		if err != nil {
			return mcp.NewToolResultError("No active session: " + err.Error()), nil
		}
		provenance := mcpProvenance("mem_save_prompt")

		err = s.CreatePrompt(ctx, uuid.NewString(), protocol.PromptCreateInput{
			SessionID: sessionID, ProjectID: project, Content: content,
			Provenance: provenance,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save prompt: " + err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Prompt saved: %q", truncate(content, 80))), nil
	}
}

func handleContext(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		project, _ := args["project"].(string)
		scope, _ := args["scope"].(string)
		topicKey, _ := args["topic_key"].(string)
		tagsRaw, _ := args["tags"].(string)
		boostRaw, _ := args["prefer_tags"].(string)
		contextText, err := s.Context(ctx, protocol.ContextBuildInput{
			ProjectID: project, Scope: scope, TopicKey: topicKey,
			Tags: parseTags(tagsRaw), PreferTags: parseTags(boostRaw),
		})
		if err != nil { return mcp.NewToolResultError("Failed to get context: " + err.Error()), nil }
		if contextText == "" { return mcp.NewToolResultText("No previous session memories found."), nil }
		stats, err := s.Stats(ctx)
		if err != nil { return mcp.NewToolResultError("Failed to get memory statistics: " + err.Error()), nil }
		projects := "none"
		if len(stats.Projects) > 0 { projects = strings.Join(stats.Projects, ", ") }
		return mcp.NewToolResultText(fmt.Sprintf("%s\n---\nMemory stats: %d sessions, %d observations across projects: %s",
			contextText, stats.TotalSessions, stats.TotalObservations, projects)), nil
	}
}

func handleGetObservation(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, _ := req.GetArguments()["ref"].(string)
		ref, err := protocol.ParseResourceRef(raw)
		if err != nil {
			return mcp.NewToolResultError("Invalid Mnemo resource reference: " + err.Error()), nil
		}
		obs, err := s.Get(ctx, ref)
		if err != nil {
			return mcp.NewToolResultError("Failed to get observation: " + err.Error()), nil
		}

		topic := ""
		if obs.TopicKey != "" {
			topic = fmt.Sprintf("\nTopic: %s", obs.TopicKey)
		}
		tool := ""
		if obs.ToolName != "" {
			tool = fmt.Sprintf("\nTool: %s", obs.ToolName)
		}
		result := fmt.Sprintf("%s [%s] %s\n%s\nSession: %s\nProject: %s\nScope: %s%s%s\nDuplicates: %d\nRevisions: %d\nCreated: %s",
			obs.Ref.String(), obs.Type, obs.Title, obs.Content,
			obs.SessionID, obs.Ref.ProjectID, obs.Scope, topic, tool,
			obs.DuplicateCount, obs.RevisionCount, obs.CreatedAt,
		)
		return mcp.NewToolResultText(result), nil
	}
}
