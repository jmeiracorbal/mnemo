package cloudsync

import "encoding/json"

// CloudBackend is the interface that cloud sync providers must implement.
// Each method maps to a direction of the sync protocol.
type CloudBackend interface {
	PushMutations(entries []MutationEntry) (*PushResult, error)
	PullMutations(sinceSeq int64, limit int) (*PullResult, error)
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

type Result struct {
	Pushed     int    `json:"pushed"`
	Pulled     int    `json:"pulled"`
	SkippedOwn int    `json:"skipped_own"`
	Pending    int    `json:"pending"`
	LatestSeq  int64  `json:"latest_seq"`
	TargetKey  string `json:"target_key"`
	Lifecycle  string `json:"lifecycle"`
}
