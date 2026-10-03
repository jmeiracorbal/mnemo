package mcp

import _ "embed"

//go:embed descriptions/server.md
var serverInstructions string

//go:embed descriptions/mem_search.md
var memSearchDescription string

//go:embed descriptions/mem_save.md
var memSaveDescription string

//go:embed descriptions/mem_suggest_topic_key.md
var memSuggestTopicKeyDescription string

//go:embed descriptions/mem_save_prompt.md
var memSavePromptDescription string

//go:embed descriptions/mem_context.md
var memContextDescription string

//go:embed descriptions/mem_list_tags.md
var memListTagsDescription string

//go:embed descriptions/mem_merge_tags.md
var memMergeTagsDescription string

//go:embed descriptions/mem_tag_stats.md
var memTagStatsDescription string

//go:embed descriptions/mem_related_tags.md
var memRelatedTagsDescription string

//go:embed descriptions/mem_timeline.md
var memTimelineDescription string

//go:embed descriptions/mem_get_observation.md
var memGetObservationDescription string

//go:embed descriptions/mem_session_summary.md
var memSessionSummaryDescription string

//go:embed descriptions/mem_capture_passive.md
var memCapturePassiveDescription string

//go:embed descriptions/mem_current_project.md
var memCurrentProjectDescription string

//go:embed descriptions/mem_doctor.md
var memDoctorDescription string
