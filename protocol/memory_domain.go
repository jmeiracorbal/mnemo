package protocol

// Memory curation returns stable references, never SQLite row identifiers.

type MemoryReviewInput struct {
	ProjectID string `json:"project_id,omitempty"`
	Scope string `json:"scope,omitempty"`
	TopicKey string `json:"topic_key,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type MemoryReviewState struct {
	State string `json:"state,omitempty"`
	Reason string `json:"reason,omitempty"`
	SupersededBy *ResourceRef `json:"superseded_by,omitempty"`
	ReviewedAt string `json:"reviewed_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type MemoryReviewObservation struct {
	Ref ResourceRef `json:"ref"`
	Type string `json:"type"`
	Title string `json:"title"`
	Scope string `json:"scope"`
	TopicKey string `json:"topic_key,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ReviewState *MemoryReviewState `json:"review_state,omitempty"`
}

type MemoryConflictGroup struct {
	ID string `json:"id"`
	Kind string `json:"kind"`
	Reason string `json:"reason"`
	Confidence float64 `json:"confidence"`
	ProjectID string `json:"project_id,omitempty"`
	Scope string `json:"scope,omitempty"`
	TopicKey string `json:"topic_key,omitempty"`
	SuggestedAction string `json:"suggested_action"`
	Observations []MemoryReviewObservation `json:"observations"`
}

type MemoryReviewOutput struct {
	Total int `json:"total"`
	Groups []MemoryConflictGroup `json:"groups"`
}

type MemoryMarkInput struct {
	Ref ResourceRef `json:"ref"`
	Reason string `json:"reason,omitempty"`
}

type MemorySupersedeInput struct {
	OldRef ResourceRef `json:"old_ref"`
	NewRef ResourceRef `json:"new_ref"`
	Reason string `json:"reason,omitempty"`
}

type MemoryConsolidationInput struct {
	FromTopic string `json:"from_topic"`
	ToTopic string `json:"to_topic"`
	ProjectID string `json:"project_id,omitempty"`
	Scope string `json:"scope,omitempty"`
}

type MemoryConsolidationOutput struct {
	FromTopic string `json:"from_topic"`
	ToTopic string `json:"to_topic"`
	ProjectID string `json:"project_id,omitempty"`
	Scope string `json:"scope,omitempty"`
	Total int `json:"total"`
	Refs []ResourceRef `json:"refs"`
}
