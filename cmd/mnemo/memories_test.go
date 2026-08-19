package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

func TestParseMemoriesConsolidateTopicRequiresSafetyFlag(t *testing.T) {
	_, err := parseMemoriesConsolidateTopicArgs([]string{"--from=old-topic", "--to=new-topic"})
	if err == nil {
		t.Fatal("expected safety flag error")
	}
	opts, err := parseMemoriesConsolidateTopicArgs([]string{"--from=old-topic", "--to=new-topic", "--dry-run", "--json"})
	if err != nil {
		t.Fatalf("parse dry-run: %v", err)
	}
	if !opts.DryRun || !opts.JSON || opts.Yes {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestPrintMemoriesReviewReportJSON(t *testing.T) {
	report := store.MemoryReviewReport{Groups: []store.MemoryConflictGroup{{
		Kind:            "duplicate-title",
		Reason:          "same title",
		Confidence:      0.65,
		SuggestedAction: "review",
		Observations: []store.MemoryConflictObservation{{
			ID:        1,
			Type:      "decision",
			Title:     "Cache strategy",
			Scope:     "project",
			CreatedAt: "2026-01-01 00:00:00",
			UpdatedAt: "2026-01-01 00:00:00",
		}},
	}}}
	report.Total = len(report.Groups)
	var out bytes.Buffer
	if err := printJSONTo(&out, report); err != nil {
		t.Fatalf("print json: %v", err)
	}
	var got store.MemoryReviewReport
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if got.Total != 1 || got.Groups[0].Kind != "duplicate-title" {
		t.Fatalf("unexpected report: %+v", got)
	}
}

func TestPrintMemoriesReviewReportText(t *testing.T) {
	report := &store.MemoryReviewReport{Total: 1, Groups: []store.MemoryConflictGroup{{
		Kind:            "topic-conflict",
		Reason:          "same topic",
		Confidence:      0.85,
		TopicKey:        "architecture/api",
		SuggestedAction: "supersede stale memory",
		Observations: []store.MemoryConflictObservation{{
			ID:        7,
			Type:      "decision",
			Title:     "Use REST",
			Scope:     "project",
			UpdatedAt: "2026-01-01 00:00:00",
		}},
	}}}
	var out bytes.Buffer
	printMemoriesReviewReport(&out, report)
	got := out.String()
	for _, want := range []string{"topic-conflict", "architecture/api", "Use REST", "supersede stale memory"} {
		if !strings.Contains(got, want) {
			t.Fatalf("text report missing %q:\n%s", want, got)
		}
	}
}
