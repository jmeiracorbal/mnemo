package store

import "testing"

func TestReviewMemoryConflictsDetectsAndResolvesDuplicateTitle(t *testing.T) {
	s := newTestStore(t)
	provenance := ProvenanceInput{AgentID: AgentCursor, SourceKindID: SourceMCP, ToolID: ToolMemSave}
	if err := s.CreateSessionWithProvenance("s-review", "alpha", "/tmp/alpha", provenance); err != nil {
		t.Fatalf("create session: %v", err)
	}
	firstID, err := s.AddObservation(AddObservationParams{
		SessionID:  "s-review",
		Type:       "decision",
		Title:      "Cache strategy",
		Content:    "Use a local cache for package metadata.",
		Project:    "alpha",
		Scope:      "project",
		Provenance: provenance,
	})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	secondID, err := s.AddObservation(AddObservationParams{
		SessionID:  "s-review",
		Type:       "decision",
		Title:      "Cache   strategy",
		Content:    "Avoid the local cache for package metadata.",
		Project:    "alpha",
		Scope:      "project",
		Provenance: provenance,
	})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}

	report, err := s.ReviewMemoryConflicts(MemoryReviewOptions{Project: "alpha"})
	if err != nil {
		t.Fatalf("review conflicts: %v", err)
	}
	if report.Total != 1 {
		t.Fatalf("conflicts = %d, want 1 (%+v)", report.Total, report.Groups)
	}
	if report.Groups[0].Kind != "duplicate-title" {
		t.Fatalf("kind = %q, want duplicate-title", report.Groups[0].Kind)
	}
	if len(report.Groups[0].Observations) != 2 {
		t.Fatalf("observations = %d, want 2", len(report.Groups[0].Observations))
	}
	if report.Groups[0].Observations[0].Provenance == nil || report.Groups[0].Observations[0].Provenance.AgentID != AgentCursor {
		t.Fatalf("review observation missing provenance: %+v", report.Groups[0].Observations[0])
	}

	if err := s.SupersedeMemory(firstID, secondID, "newer decision is canonical"); err != nil {
		t.Fatalf("supersede: %v", err)
	}
	report, err = s.ReviewMemoryConflicts(MemoryReviewOptions{Project: "alpha"})
	if err != nil {
		t.Fatalf("review after supersede: %v", err)
	}
	if report.Total != 0 {
		t.Fatalf("conflicts after supersede = %d, want 0 (%+v)", report.Total, report.Groups)
	}
}

func TestReviewMemoryConflictsDetectsTopicConflictAndConsolidatesTopic(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("s-topic", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	firstID, err := s.AddObservation(AddObservationParams{
		SessionID: "s-topic",
		Type:      "decision",
		Title:     "Use REST",
		Content:   "Use REST API endpoints.",
		Project:   "alpha",
		TopicKey:  "architecture/api",
	})
	if err != nil {
		t.Fatalf("add topic observation: %v", err)
	}
	otherTopicID, err := s.AddObservation(AddObservationParams{
		SessionID: "s-topic",
		Type:      "decision",
		Title:     "Use GraphQL",
		Content:   "Use GraphQL for the public API.",
		Project:   "alpha",
		TopicKey:  "architecture/graphql",
	})
	if err != nil {
		t.Fatalf("add other topic observation: %v", err)
	}

	plan, err := s.PlanMemoryTopicConsolidation("architecture/graphql", "architecture/api", "alpha", "")
	if err != nil {
		t.Fatalf("plan consolidation: %v", err)
	}
	if plan.Total != 1 || len(plan.IDs) != 1 || plan.IDs[0] != otherTopicID {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if _, err := s.ConsolidateMemoryTopic("architecture/graphql", "architecture/api", "alpha", ""); err != nil {
		t.Fatalf("consolidate topic: %v", err)
	}

	report, err := s.ReviewMemoryConflicts(MemoryReviewOptions{Project: "alpha", TopicKey: "architecture/api"})
	if err != nil {
		t.Fatalf("review topic conflicts: %v", err)
	}
	if report.Total != 1 || report.Groups[0].Kind != "topic-conflict" {
		t.Fatalf("unexpected topic conflict report: %+v", report)
	}
	if err := s.MarkMemoryStale(firstID, "GraphQL decision superseded REST"); err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	report, err = s.ReviewMemoryConflicts(MemoryReviewOptions{Project: "alpha", TopicKey: "architecture/api"})
	if err != nil {
		t.Fatalf("review after stale: %v", err)
	}
	if report.Total != 0 {
		t.Fatalf("conflicts after stale = %d, want 0", report.Total)
	}
}
