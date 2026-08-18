package db

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed schema.sql
var Schema string

var tableStatementPattern = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+[^;]+;`)

// ApplyTableSQL returns CREATE TABLE statements with IF NOT EXISTS, for the
// first migration pass before column backfills. Virtual FTS tables are applied
// later so ALTER TABLE does not corrupt external-content FTS.
func ApplyTableSQL() string {
	return withIfNotExists(joinStatements(tableStatements(Schema)))
}

// ApplyObjectSQL returns virtual tables, indexes, and triggers with IF NOT EXISTS.
func ApplyObjectSQL() string {
	sql := tableStatementPattern.ReplaceAllString(Schema, "")
	return withIfNotExists(strings.TrimSpace(sql) + "\n")
}

func tableStatements(sql string) []string {
	matches := tableStatementPattern.FindAllString(sql, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, strings.TrimSpace(match))
	}
	return out
}

func joinStatements(statements []string) string {
	return strings.Join(statements, "\n\n") + "\n"
}

func withIfNotExists(sql string) string {
	replacements := []struct {
		old string
		new string
	}{
		{"CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS "},
		{"CREATE INDEX ", "CREATE INDEX IF NOT EXISTS "},
		{"CREATE VIRTUAL TABLE ", "CREATE VIRTUAL TABLE IF NOT EXISTS "},
		{"CREATE TRIGGER ", "CREATE TRIGGER IF NOT EXISTS "},
		{"CREATE TABLE ", "CREATE TABLE IF NOT EXISTS "},
	}
	for _, replacement := range replacements {
		sql = strings.ReplaceAll(sql, replacement.old, replacement.new)
	}
	return sql
}
