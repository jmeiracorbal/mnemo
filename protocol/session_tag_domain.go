package protocol

import "time"

// Typed session and tag contracts are independent of transport and SQLite.

type SessionResolveInput struct {
	ProjectID string `json:"project_id"`
	Directory string `json:"directory"`
	InstanceID string `json:"instance_id,omitempty"`
	PID int `json:"pid,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	NativeID string `json:"native_id,omitempty"`
}

type SessionResolveOutput struct {
	SessionID string `json:"session_id"`
}

type SessionCloseInput struct {
	InstanceID string `json:"instance_id"`
}

type SessionBindExecutionInput struct {
	ProjectID string `json:"project_id"`
	AgentID string `json:"agent_id"`
	NativeID string `json:"native_id"`
	SessionID string `json:"session_id"`
}

type SessionIDInput struct {
	SessionID string `json:"session_id"`
}

type SessionExistsOutput struct {
	Exists bool `json:"exists"`
}

type SessionCountOutput struct {
	Count int `json:"count"`
}

type TagListInput struct {
	ProjectID string `json:"project_id"`
}

type TagInfo struct {
	Tag string `json:"tag"`
	Count int `json:"count"`
	LastUsedAt string `json:"last_used_at"`
}

type TagListOutput struct {
	Tags []TagInfo `json:"tags"`
}

type TagStatsOptions struct {
	MinCount int `json:"min_count,omitempty"`
	MaxCount int `json:"max_count,omitempty"`
	UnusedSince time.Time `json:"unused_since,omitempty"`
	Limit int `json:"limit,omitempty"`
	SortBy string `json:"sort_by,omitempty"`
}

type TagStatsInput struct {
	ProjectID string `json:"project_id"`
	Options TagStatsOptions `json:"options"`
}

type TagRelatedOptions struct {
	Limit int `json:"limit,omitempty"`
	Since time.Time `json:"since,omitempty"`
	MinCooccurrence int `json:"min_cooccurrence,omitempty"`
	IncludeObservations bool `json:"include_observations"`
	IncludeSessions bool `json:"include_sessions"`
}

type TagRelatedInput struct {
	ProjectID string `json:"project_id"`
	Tag string `json:"tag"`
	Options TagRelatedOptions `json:"options"`
}

type RelatedTag struct {
	Tag string `json:"tag"`
	CooccurrenceCount int `json:"cooccurrence_count"`
	Score float64 `json:"score"`
	LastSeenAt string `json:"last_seen_at"`
}

type TagRelatedOutput struct {
	Tags []RelatedTag `json:"tags"`
}

type TagMergeInput struct {
	From string `json:"from"`
	To string `json:"to"`
}

type TagMergeOutput struct {
	Observations int `json:"observations"`
	Sessions int `json:"sessions"`
	CanonicalTo string `json:"canonical_to"`
}

type TagStatsOutput struct {
	Tags []TagInfo `json:"tags"`
}
