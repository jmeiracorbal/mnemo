package store

import "time"

type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDeleted bool   `json:"is_deleted"`
	CreatedAt string `json:"created_at"`
}

type ProjectSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	CreatedAt        string `json:"created_at"`
	Directory        string `json:"directory,omitempty"`
	SessionCount     int    `json:"session_count"`
	ObservationCount int    `json:"observation_count"`
	PromptCount      int    `json:"prompt_count"`
	LastSeenAt       string `json:"last_seen_at"`
}

type Session struct {
	ID         string      `json:"id"`
	Project    string      `json:"project"`
	Directory  string      `json:"directory"`
	StartedAt  string      `json:"started_at"`
	EndedAt    *string     `json:"ended_at,omitempty"`
	Summary    *string     `json:"summary,omitempty"`
	IsDeleted  bool        `json:"is_deleted"`
	Tags       []string    `json:"tags,omitempty"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

type Observation struct {
	ID             int64       `json:"id"`
	SyncID         string      `json:"sync_id"`
	SessionID      string      `json:"session_id"`
	Type           string      `json:"type"`
	Title          string      `json:"title"`
	Content        string      `json:"content"`
	ToolName       *string     `json:"tool_name,omitempty"`
	Project        *string     `json:"project,omitempty"`
	Scope          string      `json:"scope"`
	TopicKey       *string     `json:"topic_key,omitempty"`
	Tags           []string    `json:"tags,omitempty"`
	RevisionCount  int         `json:"revision_count"`
	DuplicateCount int         `json:"duplicate_count"`
	LastSeenAt     *string     `json:"last_seen_at,omitempty"`
	CreatedAt      string      `json:"created_at"`
	UpdatedAt      string      `json:"updated_at"`
	IsDeleted      bool        `json:"is_deleted"`
	Provenance     *Provenance `json:"provenance,omitempty"`
}

type TagInfo struct {
	Tag        string `json:"tag"`
	Count      int    `json:"count"`
	LastUsedAt string `json:"last_used_at"`
}

// TagStatsOptions controls filtering and ordering in TagStats.
type TagStatsOptions struct {
	MinCount    int
	MaxCount    int
	UnusedSince time.Time
	Limit       int
	// SortBy controls result ordering: "freq" (default), "stale", or "alpha".
	SortBy string
}

// TagWeight holds observability signals for a single tag.
type TagWeight struct {
	Tag          string
	Frequency    int
	RecencyScore float64
	IsDominant   bool
}

type RelatedTag struct {
	Tag               string  `json:"tag"`
	CooccurrenceCount int     `json:"cooccurrence_count"`
	Score             float64 `json:"score"`
	LastSeenAt        string  `json:"last_seen_at"`
}

type RelatedTagsOptions struct {
	Limit               int
	Since               time.Time
	MinCooccurrence     int
	IncludeObservations bool
	IncludeSessions     bool
}

type SearchResult struct {
	Observation
	Rank float64 `json:"rank"`
}

type SessionSummary struct {
	ID               string      `json:"id"`
	Project          string      `json:"project"`
	StartedAt        string      `json:"started_at"`
	EndedAt          *string     `json:"ended_at,omitempty"`
	Summary          *string     `json:"summary,omitempty"`
	ObservationCount int         `json:"observation_count"`
	Provenance       *Provenance `json:"provenance,omitempty"`
}

type Stats struct {
	TotalSessions     int      `json:"total_sessions"`
	TotalObservations int      `json:"total_observations"`
	TotalPrompts      int      `json:"total_prompts"`
	Projects          []string `json:"projects"`
}

type TimelineEntry struct {
	ID             int64       `json:"id"`
	SessionID      string      `json:"session_id"`
	Type           string      `json:"type"`
	Title          string      `json:"title"`
	Content        string      `json:"content"`
	ToolName       *string     `json:"tool_name,omitempty"`
	Project        *string     `json:"project,omitempty"`
	Scope          string      `json:"scope"`
	TopicKey       *string     `json:"topic_key,omitempty"`
	RevisionCount  int         `json:"revision_count"`
	DuplicateCount int         `json:"duplicate_count"`
	LastSeenAt     *string     `json:"last_seen_at,omitempty"`
	CreatedAt      string      `json:"created_at"`
	UpdatedAt      string      `json:"updated_at"`
	IsDeleted      bool        `json:"is_deleted"`
	IsFocus        bool        `json:"is_focus"`
	Provenance     *Provenance `json:"provenance,omitempty"`
}

type TimelineResult struct {
	Focus        Observation     `json:"focus"`
	Before       []TimelineEntry `json:"before"`
	After        []TimelineEntry `json:"after"`
	SessionInfo  *Session        `json:"session_info"`
	TotalInRange int             `json:"total_in_range"`
}

type SearchOptions struct {
	Type       string   `json:"type,omitempty"`
	Project    string   `json:"project,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	TopicKey   string   `json:"topic_key,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	PreferTags []string `json:"prefer_tags,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// ContextOptions controls retrieval behaviour in RecentObservations / FormatContext.
type ContextOptions struct {
	Tags       []string
	PreferTags []string
	TopicKey   string
}

type AddObservationParams struct {
	SessionID  string          `json:"session_id"`
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	ToolName   string          `json:"tool_name,omitempty"`
	Project    string          `json:"project,omitempty"`
	Scope      string          `json:"scope,omitempty"`
	TopicKey   string          `json:"topic_key,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Provenance ProvenanceInput `json:"provenance,omitempty"`
}

type UpdateObservationParams struct {
	Type     *string   `json:"type,omitempty"`
	Title    *string   `json:"title,omitempty"`
	Content  *string   `json:"content,omitempty"`
	Project  *string   `json:"project,omitempty"`
	Scope    *string   `json:"scope,omitempty"`
	TopicKey *string   `json:"topic_key,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
}

type Prompt struct {
	ID         int64       `json:"id"`
	SyncID     string      `json:"sync_id"`
	SessionID  string      `json:"session_id"`
	Content    string      `json:"content"`
	Project    string      `json:"project,omitempty"`
	CreatedAt  string      `json:"created_at"`
	IsDeleted  bool        `json:"is_deleted"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

type AddPromptParams struct {
	SessionID  string          `json:"session_id"`
	Content    string          `json:"content"`
	Project    string          `json:"project,omitempty"`
	Provenance ProvenanceInput `json:"provenance,omitempty"`
}

type ProvenanceInput struct {
	AgentID          string `json:"agent_id,omitempty"`
	SourceKindID     string `json:"source_kind_id,omitempty"`
	ToolID           string `json:"tool_id,omitempty"`
	ModelID          string `json:"model_id,omitempty"`
	ModelProvider    string `json:"model_provider,omitempty"`
	MCPClientID      string `json:"mcp_client_id,omitempty"`
	MCPClientName    string `json:"mcp_client_name,omitempty"`
	MCPClientVersion string `json:"mcp_client_version,omitempty"`
	MCPTransport     string `json:"mcp_transport,omitempty"`
}

type Provenance struct {
	ID                 int64  `json:"id"`
	AgentID            string `json:"agent_id"`
	AgentDisplayName   string `json:"agent_display_name"`
	AgentKind          string `json:"agent_kind"`
	SourceKindID       string `json:"source_kind_id"`
	SourceDisplayName  string `json:"source_display_name"`
	ToolID             string `json:"tool_id"`
	ToolDisplayName    string `json:"tool_display_name"`
	ModelID            string `json:"model_id"`
	ModelProvider      string `json:"model_provider"`
	ModelDisplayName   string `json:"model_display_name"`
	MCPClientID        string `json:"mcp_client_id"`
	MCPClientName      string `json:"mcp_client_name"`
	MCPClientVersion   string `json:"mcp_client_version"`
	MCPClientTransport string `json:"mcp_client_transport"`
	CreatedAt          string `json:"created_at"`
}

const (
	DefaultSyncTargetKey = "cloud"
	DefaultSyncTypeID    = "cloud"

	SyncLifecycleIdle     = "idle"
	SyncLifecyclePending  = "pending"
	SyncLifecycleRunning  = "running"
	SyncLifecycleHealthy  = "healthy"
	SyncLifecycleDegraded = "degraded"

	SyncEntityProject           = "project"
	SyncEntitySession           = "session"
	SyncEntityObservation       = "observation"
	SyncEntityUserPrompt        = "user_prompt"
	SyncEntityObservationTag    = "observation_tag"
	SyncEntitySessionTag        = "session_tag"
	SyncEntityObservationReview = "observation_review"
	SyncEntityProvenanceContext = "provenance_context"
	SyncEntityAgent             = "agent"
	SyncEntityTool              = "tool"
	SyncEntityModel             = "model"
	SyncEntitySourceKind        = "source_kind"
	SyncEntityMCPClient         = "mcp_client"

	SyncOpUpsert = "upsert"

	SyncSourceLocal  = "local"
	SyncSourceRemote = "remote"
)

type SyncState struct {
	TargetKey           string  `json:"target_key"`
	SyncTypeID          string  `json:"sync_type_id"`
	Lifecycle           string  `json:"lifecycle"`
	LastEnqueuedSeq     int64   `json:"last_enqueued_seq"`
	LastAckedSeq        int64   `json:"last_acked_seq"`
	LastPulledSeq       int64   `json:"last_pulled_seq"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	BackoffUntil        *string `json:"backoff_until,omitempty"`
	LeaseOwner          *string `json:"lease_owner,omitempty"`
	LeaseUntil          *string `json:"lease_until,omitempty"`
	LastError           *string `json:"last_error,omitempty"`
	UpdatedAt           string  `json:"updated_at"`
}

type SyncMutation struct {
	Seq        int64   `json:"seq"`
	TargetKey  string  `json:"target_key"`
	Entity     string  `json:"entity"`
	EntityKey  string  `json:"entity_key"`
	Op         string  `json:"op"`
	Payload    string  `json:"payload"`
	Source     string  `json:"source"`
	OccurredAt string  `json:"occurred_at"`
	AckedAt    *string `json:"acked_at,omitempty"`
}

type syncSessionPayload struct {
	ID         string           `json:"id"`
	Project    string           `json:"project"`
	Directory  string           `json:"directory"`
	EndedAt    *string          `json:"ended_at,omitempty"`
	Summary    *string          `json:"summary,omitempty"`
	IsDeleted  bool             `json:"is_deleted"`
	Provenance *ProvenanceInput `json:"provenance,omitempty"`
}

type syncObservationPayload struct {
	SyncID     string           `json:"sync_id"`
	SessionID  string           `json:"session_id"`
	Type       string           `json:"type"`
	Title      string           `json:"title"`
	Content    string           `json:"content"`
	ToolName   *string          `json:"tool_name,omitempty"`
	Scope      string           `json:"scope"`
	TopicKey   *string          `json:"topic_key,omitempty"`
	IsDeleted  bool             `json:"is_deleted"`
	Provenance *ProvenanceInput `json:"provenance,omitempty"`
}

type syncPromptPayload struct {
	SyncID     string           `json:"sync_id"`
	SessionID  string           `json:"session_id"`
	Content    string           `json:"content"`
	IsDeleted  bool             `json:"is_deleted"`
	Provenance *ProvenanceInput `json:"provenance,omitempty"`
}

type ExportData struct {
	Version      string        `json:"version"`
	ExportedAt   string        `json:"exported_at"`
	Sessions     []Session     `json:"sessions"`
	Observations []Observation `json:"observations"`
	Prompts      []Prompt      `json:"prompts"`
}

type ImportResult struct {
	SessionsImported     int `json:"sessions_imported"`
	ObservationsImported int `json:"observations_imported"`
	PromptsImported      int `json:"prompts_imported"`
}

type MigrateResult struct {
	Migrated             bool  `json:"migrated"`
	ObservationsUpdated  int64 `json:"observations_updated"`
	SessionsUpdated      int64 `json:"sessions_updated"`
	PromptsUpdated       int64 `json:"prompts_updated"`
	SyncMutationsUpdated int64 `json:"sync_mutations_updated"`
}

type ProjectMergePlan struct {
	From                    ProjectSummary `json:"from"`
	To                      ProjectSummary `json:"to"`
	Observations            int64          `json:"observations"`
	Sessions                int64          `json:"sessions"`
	Prompts                 int64          `json:"prompts"`
	SyncMutations           int64          `json:"sync_mutations"`
	SourceProjectRows       int64          `json:"source_project_rows"`
	WillDeleteSourceProject bool           `json:"will_delete_source_project"`
}

type ProjectMergeResult struct {
	Plan                 ProjectMergePlan `json:"plan"`
	Merged               bool             `json:"merged"`
	ObservationsUpdated  int64            `json:"observations_updated"`
	SessionsUpdated      int64            `json:"sessions_updated"`
	PromptsUpdated       int64            `json:"prompts_updated"`
	SyncMutationsUpdated int64            `json:"sync_mutations_updated"`
	SourceProjectDeleted bool             `json:"source_project_deleted"`
}

type ProjectRenameSelector struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
}

type ProjectRenamePlan struct {
	Project       ProjectSummary `json:"project"`
	Selector      string         `json:"selector"`
	SelectorValue string         `json:"selector_value"`
	NewName       string         `json:"new_name"`
	WillChange    bool           `json:"will_change"`
}

type ProjectRenameResult struct {
	Plan    ProjectRenamePlan `json:"plan"`
	Renamed bool              `json:"renamed"`
}

type PassiveCaptureParams struct {
	SessionID  string          `json:"session_id"`
	Content    string          `json:"content"`
	Project    string          `json:"project,omitempty"`
	Source     string          `json:"source,omitempty"`
	Provenance ProvenanceInput `json:"provenance,omitempty"`
}

type PassiveCaptureResult struct {
	Extracted     int      `json:"extracted"`
	Saved         int      `json:"saved"`
	Duplicates    int      `json:"duplicates"`
	SuggestedTags []string `json:"suggested_tags"`
}
