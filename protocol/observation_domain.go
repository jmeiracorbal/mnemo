package protocol

// ObservationSearchInput scopes a search to a Mnemo project when ProjectID is
// present; an omitted project explicitly requests an installation-wide search.
type ObservationSearchInput struct {
	Query      string   `json:"query"`
	ProjectID  string   `json:"project_id,omitempty"`
	Type       string   `json:"type,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	TopicKey   string   `json:"topic_key,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	PreferTags []string `json:"prefer_tags,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

type ObservationMatch struct {
	Observation Observation `json:"observation"`
	Rank        float64     `json:"rank"`
}

type ObservationSearchOutput struct {
	Matches []ObservationMatch `json:"matches"`
}

// ObservationChanges can update observation content and organization. Project
// identity is intentionally not writable here; cross-project moves belong to
// project-level operations, never an accidental observation patch.
type ObservationChanges struct {
	Type     *string   `json:"type,omitempty"`
	Title    *string   `json:"title,omitempty"`
	Content  *string   `json:"content,omitempty"`
	Scope    *string   `json:"scope,omitempty"`
	TopicKey *string   `json:"topic_key,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
}

type ObservationUpdateInput struct {
	Ref     ResourceRef        `json:"ref"`
	Changes ObservationChanges `json:"changes"`
}

type ObservationDeleteInput struct {
	Ref ResourceRef `json:"ref"`
}

type ObservationTimelineInput struct {
	Ref    ResourceRef `json:"ref"`
	Before int         `json:"before,omitempty"`
	After  int         `json:"after,omitempty"`
}

type ObservationTimelineOutput struct {
	Focus        Observation   `json:"focus"`
	Before       []Observation `json:"before"`
	After        []Observation `json:"after"`
	SessionID    string        `json:"session_id"`
	SessionStart string        `json:"session_started_at"`
	SessionSummary string      `json:"session_summary,omitempty"`
	TotalInRange int           `json:"total_in_range"`
}

type ContextBuildInput struct {
	ProjectID  string   `json:"project_id,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	TopicKey   string   `json:"topic_key,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	PreferTags []string `json:"prefer_tags,omitempty"`
}

type ContextBuildOutput struct {
	Text string `json:"text"`
}

type LimitsOutput struct {
	MaxObservationLength int `json:"max_observation_length"`
}

type StatsOutput struct {
	TotalSessions     int      `json:"total_sessions"`
	TotalObservations int      `json:"total_observations"`
	TotalPrompts      int      `json:"total_prompts"`
	Projects          []string `json:"projects"`
}
