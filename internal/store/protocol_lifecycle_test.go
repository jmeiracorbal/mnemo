package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmeiracorbal/mnemo/protocol"
)

func TestPublicObservationLifecycleUsesCanonicalRefs(t *testing.T) {
	_, api, _ := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const project = "project-lifecycle"
	ref, err := api.Create(ctx, "lifecycle-create", protocol.ObservationCreateInput{
		ProjectID: project, Type: "decision", Title: "Original architecture",
		Content: "Use Mnemo as independent persistent memory",
		Tags: []string{"domain"},
	})
	if err != nil { t.Fatal(err) }
	results, err := api.Search(ctx, protocol.ObservationSearchInput{ProjectID: project})
	if err != nil || len(results) != 1 || results[0].Observation.Ref != ref {
		t.Fatalf("search does not use canonical references: %+v (%v)", results, err)
	}
	title := "Updated architecture"
	change := protocol.ObservationUpdateInput{
		Ref: ref, Changes: protocol.ObservationChanges{Title: &title},
	}
	updated, err := api.Update(ctx, "lifecycle-update", change)
	if err != nil || updated.Title != title || updated.Ref != ref {
		t.Fatalf("updated observation changed identity: %+v (%v)", updated, err)
	}
	retried, err := api.Update(ctx, "lifecycle-update", change)
	if err != nil || retried.Ref != updated.Ref || retried.Title != updated.Title {
		t.Fatalf("idempotent update replay: %+v (%v)", retried, err)
	}
	other := "conflicting content"
	change.Changes.Title = &other
	_, err = api.Update(ctx, "lifecycle-update", change)
	var coded *protocol.Error
	if !errors.As(err, &coded) || coded.Code != protocol.CodeConflict {
		t.Fatalf("conflicting request ID = %v, expected conflict", err)
	}
	timeline, err := api.Timeline(ctx, protocol.ObservationTimelineInput{Ref: ref, Before: 2, After: 2})
	if err != nil || timeline.Focus.Ref != ref {
		t.Fatalf("timeline does not use canonical reference: %+v (%v)", timeline, err)
	}
	if err := api.Delete(ctx, "lifecycle-delete", ref); err != nil { t.Fatal(err) }
	if err := api.Delete(ctx, "lifecycle-delete", ref); err != nil {
		t.Fatalf("idempotent delete replay: %v", err)
	}
	_, err = api.Get(ctx, ref)
	if !errors.As(err, &coded) || coded.Code != protocol.CodeNotFound {
		t.Fatalf("soft-deleted observation was returned: %v", err)
	}
}

func TestPublicSessionTagsPromptAndStats(t *testing.T) {
	_, api, _ := newProtocolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	input := protocol.SessionResolveInput{
		ProjectID: "project-sessions", Directory: t.TempDir(),
		InstanceID: "mcp-session-instance", PID: 123,
	}
	id, err := api.ResolveSession(ctx, "resolve-1", input)
	if err != nil || id == "" { t.Fatalf("resolve session: %q (%v)", id, err) }
	replayed, err := api.ResolveSession(ctx, "resolve-1", input)
	if err != nil || replayed != id { t.Fatalf("session replay: %q (%v)", replayed, err) }
	exists, err := api.SessionExists(ctx, id)
	if err != nil || !exists { t.Fatalf("session missing: %v (%v)", exists, err) }
	if _, err := api.Create(ctx, "tagged-create", protocol.ObservationCreateInput{
		ProjectID: input.ProjectID, SessionID: id, Type: "manual",
		Title: "Tagged memory", Content: "Use verified project context",
		Tags: []string{"old-tag"},
	}); err != nil { t.Fatal(err) }
	count, err := api.SessionObservationCount(ctx, id)
	if err != nil || count != 1 { t.Fatalf("session count = %d (%v)", count, err) }
	tags, err := api.ListTags(ctx, input.ProjectID)
	if err != nil || len(tags) == 0 { t.Fatalf("tags = %+v (%v)", tags, err) }
	merged, err := api.MergeTags(ctx, "merge-tags-1", protocol.TagMergeInput{From: "old-tag", To: "new-tag"})
	if err != nil || merged.Observations != 1 { t.Fatalf("tag merge = %+v (%v)", merged, err) }
	if err := api.CreatePrompt(ctx, "create-prompt-1", protocol.PromptCreateInput{
		ProjectID: input.ProjectID, SessionID: id, Content: "Never couple memory to consumers",
	}); err != nil { t.Fatal(err) }
	stats, err := api.Stats(ctx)
	if err != nil || stats.TotalObservations != 1 || stats.TotalPrompts != 1 {
		t.Fatalf("stats = %+v (%v)", stats, err)
	}
	if err := api.CloseSession(ctx, "close-session-1", protocol.SessionCloseInput{InstanceID: input.InstanceID}); err != nil {
		t.Fatal(err)
	}
}
