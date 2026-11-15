package protocol

// Project, prompt and passive-capture DTOs contain only Mnemo domain data.

type PromptCreateInput struct {
	ProjectID string `json:"project_id"`
	SessionID string `json:"session_id"`
	Content string `json:"content"`
	Provenance Provenance `json:"provenance,omitempty"`
}

type PromptCreateOutput struct {
	Created bool `json:"created"`
}

type CapturePassiveInput struct {
	ProjectID string `json:"project_id"`
	SessionID string `json:"session_id"`
	Content string `json:"content"`
	Source string `json:"source,omitempty"`
	Provenance Provenance `json:"provenance,omitempty"`
}

type CapturePassiveOutput struct {
	Extracted int `json:"extracted"`
	Saved int `json:"saved"`
	Duplicates int `json:"duplicates"`
	SuggestedTags []string `json:"suggested_tags"`
}

type ProjectSummary struct {
	ID string `json:"id"`
	Name string `json:"name"`
	CreatedAt string `json:"created_at"`
	Directory string `json:"directory,omitempty"`
	SessionCount int `json:"session_count"`
	ObservationCount int `json:"observation_count"`
	PromptCount int `json:"prompt_count"`
	LastSeenAt string `json:"last_seen_at"`
}

type ProjectListOutput struct {
	Projects []ProjectSummary `json:"projects"`
}

type ProjectPairInput struct {
	From string `json:"from"`
	To string `json:"to"`
}

type ProjectMergePlan struct {
	From ProjectSummary `json:"from"`
	To ProjectSummary `json:"to"`
	Observations int64 `json:"observations"`
	Sessions int64 `json:"sessions"`
	Prompts int64 `json:"prompts"`
	SourceProjectRows int64 `json:"source_project_rows"`
	WillDeleteSourceProject bool `json:"will_delete_source_project"`
}

type ProjectMergeOutput struct {
	Plan ProjectMergePlan `json:"plan"`
	Merged bool `json:"merged"`
	ObservationsUpdated int64 `json:"observations_updated"`
	SessionsUpdated int64 `json:"sessions_updated"`
	PromptsUpdated int64 `json:"prompts_updated"`
	SourceProjectDeleted bool `json:"source_project_deleted"`
}

type ProjectSelector struct {
	ID string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
}

type ProjectRenameInput struct {
	Selector ProjectSelector `json:"selector"`
	Name string `json:"name"`
}

type ProjectRenamePlan struct {
	Project ProjectSummary `json:"project"`
	Selector string `json:"selector"`
	SelectorValue string `json:"selector_value"`
	NewName string `json:"new_name"`
	WillChange bool `json:"will_change"`
}

type ProjectRenameOutput struct {
	Plan ProjectRenamePlan `json:"plan"`
	Renamed bool `json:"renamed"`
}

type ProjectIdentityMigrateInput struct {
	LegacyKey string `json:"legacy_key"`
	ProjectID string `json:"project_id"`
	Name string `json:"name"`
}

type ProjectIdentityMigrateOutput struct {
	Migrated bool `json:"migrated"`
	ObservationsUpdated int64 `json:"observations_updated"`
	SessionsUpdated int64 `json:"sessions_updated"`
	PromptsUpdated int64 `json:"prompts_updated"`
}
