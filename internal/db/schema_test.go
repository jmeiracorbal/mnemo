package db

import (
	"strings"
	"testing"
)

func TestApplyTableSQLUsesCanonicalTables(t *testing.T) {
	sql := ApplyTableSQL()
	required := []string{
		"CREATE TABLE IF NOT EXISTS sessions",
		"CREATE TABLE IF NOT EXISTS observations",
		"CREATE TABLE IF NOT EXISTS user_prompts",
		"CREATE TABLE IF NOT EXISTS sync_chunks",
		"CREATE TABLE IF NOT EXISTS sync_state",
		"CREATE TABLE IF NOT EXISTS sync_mutations",
		"CREATE TABLE IF NOT EXISTS sync_enrolled_projects",
		"CREATE TABLE IF NOT EXISTS observation_tags",
		"CREATE TABLE IF NOT EXISTS session_tags",
		"CREATE TABLE IF NOT EXISTS projects",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("ApplyTableSQL missing %q", want)
		}
	}
	if strings.Contains(sql, "CREATE INDEX") || strings.Contains(sql, "CREATE TRIGGER") || strings.Contains(sql, "CREATE VIRTUAL TABLE") {
		t.Fatal("ApplyTableSQL must not include indexes, triggers, or virtual tables")
	}
}

func TestApplyObjectSQLUsesCanonicalIndexesAndTriggers(t *testing.T) {
	sql := ApplyObjectSQL()
	required := []string{
		"CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts",
		"CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts",
		"CREATE INDEX IF NOT EXISTS idx_obs_scope",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_observations_sync_id",
		"CREATE TRIGGER IF NOT EXISTS obs_fts_insert",
		"CREATE TRIGGER IF NOT EXISTS prompt_fts_insert",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("ApplyObjectSQL missing %q", want)
		}
	}
	if strings.Contains(sql, "CREATE TABLE IF NOT EXISTS sessions") {
		t.Fatal("ApplyObjectSQL must not include base table statements")
	}
}
