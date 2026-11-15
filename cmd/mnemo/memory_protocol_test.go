package main

import (
	"strings"
	"testing"
)

func TestParseSearchArgsPreservesPublicProjectFilters(t *testing.T) {
	value, err := parseSearchArgs([]string{
		"architecture", "--project=project-1", "--type=decision",
		"--scope=project", "--limit=7",
	})
	if err != nil { t.Fatal(err) }
	if value.Query != "architecture" || value.ProjectID != "project-1" ||
		value.Type != "decision" || value.Scope != "project" || value.Limit != 7 {
		t.Fatalf("CLI search did not preserve documented options: %+v", value)
	}
	for _, args := range [][]string{
		{}, {"--project=project-1"}, {"topic", "--unknown=value"},
		{"topic", "--limit=0"}, {"topic", "--limit=not-a-number"},
	} {
		if _, err := parseSearchArgs(args); err == nil {
			t.Fatalf("invalid search syntax accepted: %q", strings.Join(args, " "))
		}
	}
}
