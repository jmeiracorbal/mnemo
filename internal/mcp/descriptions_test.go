package mcp

import (
	"encoding/json"
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

func TestMemoryWritingToolsDescribeCanonicalProjectID(t *testing.T) {
	srv, err := NewServerWithTools(nil, "test", nil)
	if err != nil {
		t.Fatalf("NewServerWithTools: %v", err)
	}

	tools := srv.ListTools()
	for _, name := range []string{"mem_save", "mem_save_prompt", "mem_session_summary", "mem_capture_passive"} {
		t.Run(name, func(t *testing.T) {
			tool, ok := tools[name]
			if !ok {
				t.Fatalf("expected tool %q to be registered", name)
			}
			schemaJSON, err := json.Marshal(tool.Tool.InputSchema)
			if err != nil {
				t.Fatalf("marshal %s input schema: %v", name, err)
			}
			var schema struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(schemaJSON, &schema); err != nil {
				t.Fatalf("unmarshal %s input schema: %v", name, err)
			}
			project, ok := schema.Properties["project"]
			if !ok {
				t.Fatal("project field missing from input schema")
			}
			if !strings.Contains(project.Description, ".mnemo marker") ||
				!strings.Contains(project.Description, "not its name or slug") ||
				!strings.Contains(project.Description, "mem_current_project") {
				t.Fatalf("project field must require the canonical .mnemo id, got %q", project.Description)
			}
		})
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

func TestAgentProfileDoesNotExposeSessionLifecycleTools(t *testing.T) {
	for _, name := range []string{"mem_session_start", "mem_session_end"} {
		if ProfileAgent[name] {
			t.Fatalf("agent profile must not expose %s; MCP owns session lifecycle", name)
		}
	}
}
