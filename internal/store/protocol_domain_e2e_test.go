package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/protocol"
)

func TestPublicProjectPlanningAndMerge(t *testing.T) {
	_, api, _ := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	create := func(project string) protocol.ResourceRef {
		t.Helper()
		ref, err := api.Create(ctx, "project-create-"+project, protocol.ObservationCreateInput{
			ProjectID: project, Type: "decision", Title: "Initial plan",
			Content: "Independent project memory",
		})
		if err != nil { t.Fatal(err) }
		return ref
	}
	_ = create("merge-target")
	from := create("merge-source")
	projects, err := api.ListProjects(ctx)
	if err != nil || len(projects) != 2 { t.Fatalf("projects = %+v (%v)", projects, err) }

	rename := protocol.ProjectRenameInput{
		Selector: protocol.ProjectSelector{ID: "merge-target"},
		Name: "Renamed project",
	}
	plan, err := api.PlanProjectRename(ctx, rename)
	if err != nil || plan.NewName != "Renamed project" { t.Fatalf("project rename plan: %+v (%v)", plan, err) }
	renamed, err := api.RenameProject(ctx, "rename-request-1", rename)
	if err != nil || !renamed.Renamed { t.Fatalf("project rename: %+v (%v)", renamed, err) }

	pair := protocol.ProjectPairInput{From: "merge-source", To: "merge-target"}
	mergePlan, err := api.PlanProjectMerge(ctx, pair)
	if err != nil || mergePlan.From.ID != pair.From || mergePlan.To.ID != pair.To {
		t.Fatalf("project merge plan: %+v (%v)", mergePlan, err)
	}
	result, err := api.MergeProjects(ctx, "merge-request-1", pair)
	if err != nil || !result.Merged { t.Fatalf("project merge: %+v (%v)", result, err) }
	moved := protocol.ResourceRef{ProjectID: pair.To, SyncID: from.SyncID}
	recovered, err := api.Get(ctx, moved)
	if err != nil || recovered.Ref != moved { t.Fatalf("memory moved to wrong project: %+v (%v)", recovered, err) }
	_, err = api.Get(ctx, from)
	var coded *protocol.Error
	if !errors.As(err, &coded) || coded.Code != protocol.CodeProjectMismatch {
		t.Fatalf("outdated source-project reference accepted: %v", err)
	}
}

func TestPublicMemoryCurationAndTopicConsolidation(t *testing.T) {
	_, api, _ := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first, err := api.Create(ctx, "curation-create-first", protocol.ObservationCreateInput{
		ProjectID: "curation-project", Type: "decision", Title: "Old design",
		Content: "An older architecture", TopicKey: "topic-old",
	})
	if err != nil { t.Fatal(err) }
	second, err := api.Create(ctx, "curation-create-second", protocol.ObservationCreateInput{
		ProjectID: "curation-project", Type: "decision", Title: "New design",
		Content: "A newer architecture", TopicKey: "topic-old",
	})
	if err != nil { t.Fatal(err) }
	if err := api.MarkMemoryReviewed(ctx, "review-first", protocol.MemoryMarkInput{
		Ref: first, Reason: "Evaluated against the newer decision",
	}); err != nil { t.Fatal(err) }
	if err := api.SupersedeMemory(ctx, "supersede-first", protocol.MemorySupersedeInput{
		OldRef: first, NewRef: second, Reason: "The new design replaces the old",
	}); err != nil { t.Fatal(err) }
	_, err = api.ReviewMemories(ctx, protocol.MemoryReviewInput{ProjectID: first.ProjectID, Limit: 20})
	if err != nil { t.Fatal(err) }
	planInput := protocol.MemoryConsolidationInput{
		FromTopic: "topic-old", ToTopic: "topic-new", ProjectID: first.ProjectID,
	}
	plan, err := api.PlanMemoryConsolidation(ctx, planInput)
	if err != nil || plan.Total < 1 { t.Fatalf("invalid consolidation plan: %+v (%v)", plan, err) }
	changed, err := api.ConsolidateMemories(ctx, "topic-consolidate-1", planInput)
	if err != nil || changed.Total != plan.Total { t.Fatalf("consolidation: %+v (%v)", changed, err) }
	obs, err := api.Get(ctx, second)
	if err != nil || obs.TopicKey != "topic-new" {
		t.Fatalf("consolidated topic not persisted: %+v (%v)", obs, err)
	}
}

func TestPublicArchiveRoundTripBetweenControllers(t *testing.T) {
	_, source, _ := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ref, err := source.Create(ctx, "archive-create", protocol.ObservationCreateInput{
		ProjectID: "archive-project", Type: "instruction", Title: "Portable memory",
		Content: "Reference must remain stable after import",
	})
	if err != nil { t.Fatal(err) }
	archive, err := source.Export(ctx)
	if err != nil || archive.Version != protocol.Version {
		t.Fatalf("archive export: %+v (%v)", archive, err)
	}
	_, destination, _ := newProtocolFixture(t)
	imported, err := destination.Import(ctx, "archive-import-1", protocol.ArchiveImportInput{Archive: archive})
	if err != nil || imported.ObservationsImported != 1 {
		t.Fatalf("archive import: %+v (%v)", imported, err)
	}
	recovered, err := destination.Get(ctx, ref)
	if err != nil || recovered.Ref != ref || recovered.Content != "Reference must remain stable after import" {
		t.Fatalf("import did not preserve Mnemo identity: %+v (%v)", recovered, err)
	}
	replayed, err := destination.Import(ctx, "archive-import-1", protocol.ArchiveImportInput{Archive: archive})
	if err != nil || replayed != imported {
		t.Fatalf("idempotent archive import failed: %+v (%v)", replayed, err)
	}
}
