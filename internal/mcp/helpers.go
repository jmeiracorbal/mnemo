package mcp

import (
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

var suggestTopicKey = store.SuggestTopicKey
var suggestTags = store.SuggestTags

// parseTags splits a comma-separated tag string and returns individual values.
// Empty strings and blank tokens are dropped.
func parseTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func intArg(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
