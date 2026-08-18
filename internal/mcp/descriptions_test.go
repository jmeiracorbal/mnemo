package mcp

import (
	"strings"
	"testing"
)

func TestEmbeddedToolDescriptionsAreRegistered(t *testing.T) {
	srv, err := NewServerWithTools(nil, "test", nil)
	if err != nil {
		t.Fatalf("NewServerWithTools: %v", err)
	}
	tools := srv.ListTools()

	cases := map[string]string{
		"mem_search":            memSearchDescription,
		"mem_save":              memSaveDescription,
		"mem_suggest_topic_key": memSuggestTopicKeyDescription,
		"mem_save_prompt":       memSavePromptDescription,
		"mem_context":           memContextDescription,
		"mem_list_tags":         memListTagsDescription,
		"mem_merge_tags":        memMergeTagsDescription,
		"mem_tag_stats":         memTagStatsDescription,
		"mem_related_tags":      memRelatedTagsDescription,
		"mem_timeline":          memTimelineDescription,
		"mem_get_observation":   memGetObservationDescription,
		"mem_session_summary":   memSessionSummaryDescription,
		"mem_capture_passive":   memCapturePassiveDescription,
	}

	for name, want := range cases {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("expected tool %q to be registered", name)
		}
		if got := tool.Tool.Description; got != strings.TrimSpace(want) {
			t.Fatalf("%s description mismatch\nwant:\n%s\n\ngot:\n%s", name, strings.TrimSpace(want), got)
		}
	}
}

func TestNewServerWithToolsRequiresVersion(t *testing.T) {
	if _, err := NewServerWithTools(nil, "", nil); err == nil {
		t.Fatal("expected error when version is empty")
	}
	if _, err := NewServerWithTools(nil, "   ", nil); err == nil {
		t.Fatal("expected error when version is whitespace")
	}
}
