package mcp

import (
	"context"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/client"
	"github.com/jmeiracorbal/mnemo/protocol"
	"github.com/jmeiracorbal/mnemo/internal/adapters/agents"
	"github.com/jmeiracorbal/mnemo/internal/events"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

// MemoryBackend isolates MCP from SQLite. Store is used only by the global
// controller; MCP uses ControllerBackend and reaches it over NATS.
type MemoryBackend interface {
	Search(string, store.SearchOptions) ([]store.SearchResult, error)
	ResolveMCPInstanceSession(string, string, string, int) (string, error)
	CloseMCPInstanceSessions(string) error
	BindExecutionSession(string, agents.Agent, string, string) error
	CreateObservation(context.Context, string, protocol.ObservationCreateInput) (protocol.ResourceRef, error)
	UpdateObservation(int64, store.UpdateObservationParams) (*store.Observation, error)
	DeleteObservation(int64) error
	AddPrompt(store.AddPromptParams) (int64, error)
	PassiveCapture(store.PassiveCaptureParams) (*store.PassiveCaptureResult, error)
	FormatContextOpts(string, string, store.ContextOptions) (string, error)
	ListTags(string) ([]store.TagInfo, error)
	TagStats(string, store.TagStatsOptions) ([]store.TagInfo, error)
	MergeTags(string, string) (int, int, error)
	RelatedTags(string, string, store.RelatedTagsOptions) ([]store.RelatedTag, error)
	Stats() (*store.Stats, error)
	Timeline(int64, int, int) (*store.TimelineResult, error)
	GetObservationRef(context.Context, protocol.ResourceRef) (protocol.Observation, error)
	MaxObservationLength() int
}

// ControllerBackend implements MemoryBackend without opening SQLite.
type ControllerBackend struct {
	cfg events.Config
	api *client.Client
}

func NewControllerBackend(cfg events.Config) (*ControllerBackend, error) {
	api, err := client.New(filepath.Dir(cfg.DataDir))
	if err != nil {
		return nil, err
	}
	return &ControllerBackend{cfg: cfg, api: api}, nil
}

func (b *ControllerBackend) Health(ctx context.Context) error {
	return b.api.Health(ctx)
}

func (b *ControllerBackend) call(action string, input, output any) error {
	return events.Call(context.Background(), b.cfg, action, input, output)
}

func (b *ControllerBackend) Search(query string, options store.SearchOptions) ([]store.SearchResult, error) {
	var out []store.SearchResult
	return out, b.call("search", struct {
		Query   string              `json:"query"`
		Options store.SearchOptions `json:"options"`
	}{query, options}, &out)
}
func (b *ControllerBackend) ResolveMCPInstanceSession(project, directory, instance string, pid int) (string, error) {
	var out string
	return out, b.call("resolve_session", struct {
		Project    string `json:"project"`
		Directory  string `json:"directory"`
		InstanceID string `json:"instance_id"`
		PID        int    `json:"pid"`
	}{project, directory, instance, pid}, &out)
}
func (b *ControllerBackend) CloseMCPInstanceSessions(instance string) error {
	return b.call("close_sessions", struct {
		InstanceID string `json:"instance_id"`
	}{instance}, nil)
}
func (b *ControllerBackend) BindExecutionSession(project string, agent agents.Agent, nativeID, sessionID string) error {
	return b.call("bind_execution_session", struct {
		Project   string        `json:"project"`
		Agent     agents.Agent  `json:"agent"`
		NativeID  string         `json:"native_id"`
		SessionID string         `json:"session_id"`
	}{project, agent, nativeID, sessionID}, nil)
}
func (b *ControllerBackend) CreateObservation(ctx context.Context, requestID string, input protocol.ObservationCreateInput) (protocol.ResourceRef, error) {
	return b.api.Create(ctx, requestID, input)
}
func (b *ControllerBackend) UpdateObservation(id int64, params store.UpdateObservationParams) (*store.Observation, error) {
	var out store.Observation
	err := b.call("update_observation", struct {
		ID     int64                         `json:"id"`
		Params store.UpdateObservationParams `json:"params"`
	}{id, params}, &out)
	return &out, err
}
func (b *ControllerBackend) DeleteObservation(id int64) error {
	return b.call("delete_observation", struct {
		ID int64 `json:"id"`
	}{id}, nil)
}
func (b *ControllerBackend) AddPrompt(params store.AddPromptParams) (int64, error) {
	var out int64
	return out, b.call("add_prompt", params, &out)
}
func (b *ControllerBackend) PassiveCapture(params store.PassiveCaptureParams) (*store.PassiveCaptureResult, error) {
	var out store.PassiveCaptureResult
	err := b.call("passive_capture", params, &out)
	return &out, err
}
func (b *ControllerBackend) FormatContextOpts(project, scope string, options store.ContextOptions) (string, error) {
	var out string
	return out, b.call("format_context", struct {
		Project string               `json:"project"`
		Scope   string               `json:"scope"`
		Options store.ContextOptions `json:"options"`
	}{project, scope, options}, &out)
}
func (b *ControllerBackend) ListTags(project string) ([]store.TagInfo, error) {
	var out []store.TagInfo
	return out, b.call("list_tags", struct {
		Project string `json:"project"`
	}{project}, &out)
}
func (b *ControllerBackend) TagStats(project string, options store.TagStatsOptions) ([]store.TagInfo, error) {
	var out []store.TagInfo
	return out, b.call("tag_stats", struct {
		Project string                `json:"project"`
		Options store.TagStatsOptions `json:"options"`
	}{project, options}, &out)
}
func (b *ControllerBackend) MergeTags(from, to string) (int, int, error) {
	var out struct {
		Observations int `json:"observations"`
		Sessions     int `json:"sessions"`
	}
	err := b.call("merge_tags", struct {
		From string `json:"from"`
		To   string `json:"to"`
	}{from, to}, &out)
	return out.Observations, out.Sessions, err
}
func (b *ControllerBackend) RelatedTags(project, tag string, options store.RelatedTagsOptions) ([]store.RelatedTag, error) {
	var out []store.RelatedTag
	return out, b.call("related_tags", struct {
		Project string                   `json:"project"`
		Tag     string                   `json:"tag"`
		Options store.RelatedTagsOptions `json:"options"`
	}{project, tag, options}, &out)
}
func (b *ControllerBackend) Stats() (*store.Stats, error) {
	var out store.Stats
	err := b.call("stats", nil, &out)
	return &out, err
}
func (b *ControllerBackend) Timeline(id int64, before, after int) (*store.TimelineResult, error) {
	var out store.TimelineResult
	err := b.call("timeline", struct {
		ObservationID int64 `json:"observation_id"`
		Before        int   `json:"before"`
		After         int   `json:"after"`
	}{id, before, after}, &out)
	return &out, err
}
func (b *ControllerBackend) GetObservationRef(ctx context.Context, ref protocol.ResourceRef) (protocol.Observation, error) {
	return b.api.Get(ctx, ref)
}
func (b *ControllerBackend) MaxObservationLength() int {
	var out int
	if err := b.call("max_observation_length", nil, &out); err != nil {
		return 0
	}
	return out
}

var _ MemoryBackend = (*ControllerBackend)(nil)
