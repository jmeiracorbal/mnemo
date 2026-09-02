package cloudsync

import "encoding/json"

// mutationSessionPayload mirrors store.syncSessionPayload for JSON decoding
// without creating an import cycle.
type mutationSessionPayload struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	Directory string    `json:"directory"`
	EndedAt   *string   `json:"ended_at,omitempty"`
	Summary   *string   `json:"summary,omitempty"`
	Tags      *[]string `json:"tags,omitempty"`
}

// mutationObservationPayload mirrors store.syncObservationPayload.
type mutationObservationPayload struct {
	SyncID    string    `json:"sync_id"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	ToolName  *string   `json:"tool_name,omitempty"`
	Project   *string   `json:"project,omitempty"`
	Scope     string    `json:"scope"`
	TopicKey  *string   `json:"topic_key,omitempty"`
	Tags      *[]string `json:"tags,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`
	DeletedAt *string   `json:"deleted_at,omitempty"`
}

// mutationPromptPayload mirrors store.syncPromptPayload.
type mutationPromptPayload struct {
	SyncID    string  `json:"sync_id"`
	SessionID string  `json:"session_id"`
	Content   string  `json:"content"`
	Project   *string `json:"project,omitempty"`
}

// Cloud data table row types — written to the cloud database.

type cloudSessionRow struct {
	ID        string   `json:"id"`
	Project   string   `json:"project"`
	Directory string   `json:"directory"`
	EndedAt   *string  `json:"ended_at"`
	Summary   *string  `json:"summary"`
	Tags      []string `json:"tags"`
}

type cloudObservationRow struct {
	SyncID    string   `json:"sync_id"`
	SessionID string   `json:"session_id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	ToolName  *string  `json:"tool_name"`
	Project   *string  `json:"project"`
	Scope     string   `json:"scope"`
	TopicKey  *string  `json:"topic_key"`
	Tags      []string `json:"tags"`
	DeletedAt *string  `json:"deleted_at"`
}

type cloudPromptRow struct {
	SyncID    string  `json:"sync_id"`
	SessionID string  `json:"session_id"`
	Content   string  `json:"content"`
	Project   *string `json:"project"`
}

func derefStringSlice(s *[]string) []string {
	if s == nil {
		return []string{}
	}
	return *s
}

type MutationEntry struct {
	LocalSeq   int64           `json:"-"`
	Project    string          `json:"project"`
	Entity     string          `json:"entity"`
	EntityKey  string          `json:"entity_key"`
	Op         string          `json:"op"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt string          `json:"occurred_at,omitempty"`
}

type PushResult struct {
	AcceptedSeqs []int64 `json:"accepted_seqs"`
}

type PulledMutation struct {
	Seq        int64           `json:"seq"`
	OriginID   string          `json:"origin_id"`
	ClientSeq  int64           `json:"client_seq"`
	Project    string          `json:"project"`
	Entity     string          `json:"entity"`
	EntityKey  string          `json:"entity_key"`
	Op         string          `json:"op"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt string          `json:"occurred_at"`
}

type PullResult struct {
	Mutations []PulledMutation `json:"mutations"`
	HasMore   bool             `json:"has_more"`
	LatestSeq int64            `json:"latest_seq"`
}

type CloudBackend interface {
	PushMutations(entries []MutationEntry) (*PushResult, error)
	PullMutations(sinceSeq int64, limit int) (*PullResult, error)
}

type Result struct {
	Pushed     int    `json:"pushed"`
	Pulled     int    `json:"pulled"`
	SkippedOwn int    `json:"skipped_own"`
	Pending    int    `json:"pending"`
	LatestSeq  int64  `json:"latest_seq"`
	TargetKey  string `json:"target_key"`
	Lifecycle  string `json:"lifecycle"`
}
