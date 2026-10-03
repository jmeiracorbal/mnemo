package mcp

import "strings"

// ProfileAgent contains the tool names that AI agents need during coding sessions.
var ProfileAgent = map[string]bool{
	"mem_save":              true,
	"mem_search":            true,
	"mem_context":           true,
	"mem_session_summary":   true,
	"mem_get_observation":   true,
	"mem_suggest_topic_key": true,
	"mem_capture_passive":   true,
	"mem_save_prompt":       true,
	"mem_update":            true,
	"mem_list_tags":         true,
	"mem_merge_tags":        true,
	"mem_tag_stats":         true,
	"mem_related_tags":      true,
	"mem_current_project":   true,
	"mem_doctor":            true,
}

// ProfileAdmin contains tools for CLI curation and dashboards.
var ProfileAdmin = map[string]bool{
	"mem_delete":   true,
	"mem_stats":    true,
	"mem_timeline": true,
}

// Profiles maps profile names to their tool sets.
var Profiles = map[string]map[string]bool{
	"agent": ProfileAgent,
	"admin": ProfileAdmin,
}

// ResolveTools takes a comma-separated string of profile names and/or
// individual tool names and returns the set of tool names to register.
// An empty input means "all" — every tool is registered.
func ResolveTools(input string) map[string]bool {
	input = strings.TrimSpace(input)
	if input == "" || input == "all" {
		return nil
	}

	result := make(map[string]bool)
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == "all" {
			return nil
		}
		if profile, ok := Profiles[token]; ok {
			for tool := range profile {
				result[tool] = true
			}
		} else {
			result[token] = true
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
