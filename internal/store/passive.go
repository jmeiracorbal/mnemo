package store

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

var learningHeaderPattern = regexp.MustCompile(
	`(?im)^#{2,3}\s+(?:Aprendizajes(?:\s+Clave)?|Key\s+Learnings?|Learnings?):?\s*$`,
)

const (
	minLearningLength = 20
	minLearningWords  = 4
)

// ExtractLearnings parses structured learning items from text.
func ExtractLearnings(text string) []string {
	matches := learningHeaderPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	for i := len(matches) - 1; i >= 0; i-- {
		sectionStart := matches[i][1]
		sectionText := text[sectionStart:]

		if nextHeader := regexp.MustCompile(`\n#{1,3} `).FindStringIndex(sectionText); nextHeader != nil {
			sectionText = sectionText[:nextHeader[0]]
		}

		var learnings []string

		numbered := regexp.MustCompile(`(?m)^\s*\d+[.)]\s+(.+)`).FindAllStringSubmatch(sectionText, -1)
		if len(numbered) > 0 {
			for _, m := range numbered {
				cleaned := cleanMarkdown(m[1])
				if len(cleaned) >= minLearningLength && len(strings.Fields(cleaned)) >= minLearningWords {
					learnings = append(learnings, cleaned)
				}
			}
		}

		if len(learnings) == 0 {
			bullets := regexp.MustCompile(`(?m)^\s*[-*]\s+(.+)`).FindAllStringSubmatch(sectionText, -1)
			for _, m := range bullets {
				cleaned := cleanMarkdown(m[1])
				if len(cleaned) >= minLearningLength && len(strings.Fields(cleaned)) >= minLearningWords {
					learnings = append(learnings, cleaned)
				}
			}
		}

		if len(learnings) > 0 {
			return learnings
		}
	}

	return nil
}

func cleanMarkdown(text string) string {
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// PassiveCapture extracts learnings from text and saves them as observations.
func (s *Store) PassiveCapture(p PassiveCaptureParams) (*PassiveCaptureResult, error) {
	result := &PassiveCaptureResult{}

	learnings := ExtractLearnings(p.Content)
	result.Extracted = len(learnings)

	if len(learnings) == 0 {
		return result, nil
	}

	seenTags := make(map[string]bool)
	for _, learning := range learnings {
		for _, tag := range SuggestTags("passive", learning, learning) {
			if !seenTags[tag] {
				result.SuggestedTags = append(result.SuggestedTags, tag)
				seenTags[tag] = true
			}
		}
	}

	for _, learning := range learnings {
		normHash := hashNormalized(learning)
		_, err := s.q.FindObservationByHashAndProject(context.Background(), dbgen.FindObservationByHashAndProjectParams{
			NormalizedHash: sqlNullString(normHash), Project: sqlNullString(p.Project),
		})

		if err == nil {
			result.Duplicates++
			continue
		}

		title := learning
		if len(title) > 60 {
			title = title[:60] + "..."
		}

		_, err = s.AddObservation(AddObservationParams{
			SessionID: p.SessionID,
			Type:      "passive",
			Title:     title,
			Content:   learning,
			Project:   p.Project,
			Scope:     "project",
			ToolName:  p.Source,
		})
		if err != nil {
			return result, fmt.Errorf("passive capture save: %w", err)
		}
		result.Saved++
	}

	return result, nil
}

// SuggestTopicKey generates a stable topic key from type/title/content.
func SuggestTopicKey(typ, title, content string) string {
	family := inferTopicFamily(typ, title, content)
	cleanTitle := stripPrivateTags(title)
	segment := normalizeTopicSegment(cleanTitle)

	if segment == "" {
		cleanContent := stripPrivateTags(content)
		words := strings.Fields(strings.ToLower(cleanContent))
		if len(words) > 8 {
			words = words[:8]
		}
		segment = normalizeTopicSegment(strings.Join(words, " "))
	}

	if segment == "" {
		segment = "general"
	}

	if strings.HasPrefix(segment, family+"-") {
		segment = strings.TrimPrefix(segment, family+"-")
	}
	if segment == "" || segment == family {
		segment = "general"
	}

	return family + "/" + segment
}

// SuggestTags generates tag suggestions from type/title/content.
func SuggestTags(typ, title, content string) []string {
	family := inferTopicFamily(typ, title, content)
	seen := make(map[string]bool)
	var tags []string

	if family != "" && !isBlockedTag(family) {
		seen[family] = true
		tags = append(tags, family)
	}

	if normTyp := normalizeTag(typ); normTyp != "" && !isBlockedTag(normTyp) && !seen[normTyp] {
		tags = append(tags, normTyp)
		seen[normTyp] = true
	}

	text := strings.ToLower(title + " " + content)
	tokens := make(map[string]struct{})
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		tokens[tok] = struct{}{}
	}
	candidates := []string{
		"auth", "authentication", "authorization",
		"database", "sql", "sqlite", "postgres", "mysql",
		"api", "rest", "http", "grpc",
		"test", "testing",
		"cache", "redis",
		"config", "configuration",
		"deploy", "deployment", "ci",
		"security", "crypto",
		"performance", "latency",
		"refactor",
		"migration",
		"sync",
		"session",
		"memory",
		"error", "panic",
	}
	for _, kw := range candidates {
		if _, ok := tokens[kw]; !ok {
			continue
		}
		norm := normalizeTag(kw)
		if norm == "" || isBlockedTag(norm) || seen[norm] {
			continue
		}
		tags = append(tags, norm)
		seen[norm] = true
		if len(tags) >= 5 {
			break
		}
	}
	return tags
}

// ClassifyTool returns the observation type for a given tool name.
func ClassifyTool(toolName string) string {
	switch toolName {
	case "write", "edit", "patch":
		return "file_change"
	case "bash":
		return "command"
	case "read", "view":
		return "file_read"
	case "grep", "glob", "ls":
		return "search"
	default:
		return "tool_use"
	}
}

func inferTopicFamily(typ, title, content string) string {
	t := strings.TrimSpace(strings.ToLower(typ))
	switch t {
	case "architecture", "design", "adr", "refactor":
		return "architecture"
	case "bug", "bugfix", "fix", "incident", "hotfix":
		return "bug"
	case "decision":
		return "decision"
	case "pattern", "convention", "guideline":
		return "pattern"
	case "config", "setup", "infra", "infrastructure", "ci":
		return "config"
	case "discovery", "investigation", "root_cause", "root-cause":
		return "discovery"
	case "learning", "learn":
		return "learning"
	case "session_summary":
		return "session"
	}

	text := strings.ToLower(title + " " + content)
	if hasAny(text, "bug", "fix", "panic", "error", "crash", "regression", "incident", "hotfix") {
		return "bug"
	}
	if hasAny(text, "architecture", "design", "adr", "boundary", "hexagonal", "refactor") {
		return "architecture"
	}
	if hasAny(text, "decision", "tradeoff", "chose", "choose", "decide") {
		return "decision"
	}
	if hasAny(text, "pattern", "convention", "naming", "guideline") {
		return "pattern"
	}
	if hasAny(text, "config", "setup", "environment", "env", "docker", "pipeline") {
		return "config"
	}
	if hasAny(text, "discovery", "investigate", "investigation", "found", "root cause") {
		return "discovery"
	}
	if hasAny(text, "learned", "learning") {
		return "learning"
	}

	if t != "" && t != "manual" {
		return normalizeTopicSegment(t)
	}

	return "topic"
}
