package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func handleSearch(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, _ := req.GetArguments()["query"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		project, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		limit := intArg(req, "limit", 10)
		tagsRaw, _ := req.GetArguments()["tags"].(string)
		boostTagsRaw, _ := req.GetArguments()["prefer_tags"].(string)

		results, err := s.Search(query, store.SearchOptions{
			Type:       typ,
			Project:    project,
			Scope:      scope,
			TopicKey:   topicKey,
			Tags:       parseTags(tagsRaw),
			PreferTags: parseTags(boostTagsRaw),
			Limit:      limit,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search error: %s. Try simpler keywords.", err)), nil
		}

		if len(results) == 0 {
			if query == "" {
				return mcp.NewToolResultText("No memories found matching the specified filters."), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("No memories found for: %q", query)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d memories:\n\n", len(results))
		anyTruncated := false
		for i, r := range results {
			proj := ""
			if r.Project != nil {
				proj = fmt.Sprintf(" | project: %s", *r.Project)
			}
			preview := truncate(r.Content, 300)
			if len(r.Content) > 300 {
				anyTruncated = true
				preview += " [truncated — use mem_get_observation(id) for full content]"
			}
			fmt.Fprintf(&b, "[%d] #%d (%s) — %s\n    %s\n    %s%s | scope: %s\n\n",
				i+1, r.ID, r.Type, r.Title,
				preview,
				r.CreatedAt, proj, r.Scope)
		}
		if anyTruncated {
			fmt.Fprintf(&b, "---\nResults above are previews (300 chars). To read the full content of a specific memory, call mem_get_observation(id: <ID>).\n")
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
		provenance := store.MCPProvenance(store.ToolMemSave)

		truncated := len(content) > s.MaxObservationLength()

		_, err = s.AddObservation(store.AddObservationParams{
			SessionID:  sessionID,
			Type:       typ,
			Title:      title,
			Content:    content,
			Project:    project,
			Scope:      scope,
			TopicKey:   topicKey,
			Tags:       tags,
			Provenance: provenance,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Memory saved: %q (%s)", title, typ)
		if topicKey == "" && suggestedTopicKey != "" {
			msg += fmt.Sprintf("\nSuggested topic_key: %s", suggestedTopicKey)
		}
		if len(tags) == 0 {
			if suggested := suggestTags(typ, title, content); len(suggested) > 0 {
				msg += fmt.Sprintf("\nSuggested tags: %s", strings.Join(suggested, ", "))
			}
		}
		if truncated {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", len(content), s.MaxObservationLength())
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
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		update := store.UpdateObservationParams{}
		if v, ok := req.GetArguments()["title"].(string); ok {
			update.Title = &v
		}
		if v, ok := req.GetArguments()["content"].(string); ok {
			update.Content = &v
		}
		if v, ok := req.GetArguments()["type"].(string); ok {
			update.Type = &v
		}
		if v, ok := req.GetArguments()["project"].(string); ok {
			update.Project = &v
		}
		if v, ok := req.GetArguments()["scope"].(string); ok {
			update.Scope = &v
		}
		if v, ok := req.GetArguments()["topic_key"].(string); ok {
			update.TopicKey = &v
		}
		if v, ok := req.GetArguments()["tags"].(string); ok {
			tags := parseTags(v)
			update.Tags = &tags
		}

		if update.Title == nil && update.Content == nil && update.Type == nil && update.Project == nil && update.Scope == nil && update.TopicKey == nil && update.Tags == nil {
			return mcp.NewToolResultError("provide at least one field to update"), nil
		}

		var contentLen int
		if update.Content != nil {
			contentLen = len(*update.Content)
		}

		obs, err := s.UpdateObservation(id, update)
		if err != nil {
			return mcp.NewToolResultError("Failed to update memory: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Memory updated: #%d %q (%s, scope=%s)", obs.ID, obs.Title, obs.Type, obs.Scope)
		if contentLen > s.MaxObservationLength() {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", contentLen, s.MaxObservationLength())
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func handleDelete(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		if err := s.DeleteObservation(id); err != nil {
			return mcp.NewToolResultError("Failed to delete memory: " + err.Error()), nil
		}

		mode := "soft-deleted"
		return mcp.NewToolResultText(fmt.Sprintf("Memory #%d %s", id, mode)), nil
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
		provenance := store.MCPProvenance(store.ToolMemSavePrompt)

		_, err = s.AddPrompt(store.AddPromptParams{
			SessionID:  sessionID,
			Content:    content,
			Project:    project,
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
		project, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		tagsRaw, _ := req.GetArguments()["tags"].(string)
		boostTagsRaw, _ := req.GetArguments()["prefer_tags"].(string)

		ctx2, err := s.FormatContextOpts(project, scope, store.ContextOptions{
			Tags:       parseTags(tagsRaw),
			PreferTags: parseTags(boostTagsRaw),
			TopicKey:   topicKey,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to get context: " + err.Error()), nil
		}

		if ctx2 == "" {
			return mcp.NewToolResultText("No previous session memories found."), nil
		}

		stats, _ := s.Stats()
		var projects string
		if len(stats.Projects) > 0 {
			projects = strings.Join(stats.Projects, ", ")
		} else {
			projects = "none"
		}

		result := fmt.Sprintf("%s\n---\nMemory stats: %d sessions, %d observations across projects: %s",
			ctx2, stats.TotalSessions, stats.TotalObservations, projects)

		return mcp.NewToolResultText(result), nil
	}
}

func handleGetObservation(s MemoryBackend) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		obs, err := s.GetObservation(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Observation #%d not found", id)), nil
		}

		project := ""
		if obs.Project != nil {
			project = fmt.Sprintf("\nProject: %s", *obs.Project)
		}
		scope := fmt.Sprintf("\nScope: %s", obs.Scope)
		topic := ""
		if obs.TopicKey != nil {
			topic = fmt.Sprintf("\nTopic: %s", *obs.TopicKey)
		}
		toolName := ""
		if obs.ToolName != nil {
			toolName = fmt.Sprintf("\nTool: %s", *obs.ToolName)
		}
		duplicateMeta := fmt.Sprintf("\nDuplicates: %d", obs.DuplicateCount)
		revisionMeta := fmt.Sprintf("\nRevisions: %d", obs.RevisionCount)

		result := fmt.Sprintf("#%d [%s] %s\n%s\nSession: %s%s%s\nCreated: %s",
			obs.ID, obs.Type, obs.Title,
			obs.Content,
			obs.SessionID, project+scope+topic, toolName+duplicateMeta+revisionMeta,
			obs.CreatedAt,
		)

		return mcp.NewToolResultText(result), nil
	}
}
