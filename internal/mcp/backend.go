package mcp

import (
	"context"
	"path/filepath"

	"github.com/jmeiracorbal/mnemo/v2/client"
	"github.com/jmeiracorbal/mnemo/v2/internal/events"
	"github.com/jmeiracorbal/mnemo/v2/protocol"
)

// MemoryBackend exposes only the Mnemo public domain contract to MCP.
// No MCP consumer needs SQLite, store types or action-string RPCs.
type MemoryBackend interface {
	Search(context.Context, protocol.ObservationSearchInput) ([]protocol.ObservationMatch, error)
	ResolveSession(context.Context, string, protocol.SessionResolveInput) (string, error)
	CloseSession(context.Context, string, protocol.SessionCloseInput) error
	BindExecutionSession(context.Context, string, protocol.SessionBindExecutionInput) error
	Create(context.Context, string, protocol.ObservationCreateInput) (protocol.ResourceRef, error)
	Update(context.Context, string, protocol.ObservationUpdateInput) (protocol.Observation, error)
	Delete(context.Context, string, protocol.ResourceRef) error
	CreatePrompt(context.Context, string, protocol.PromptCreateInput) error
	CapturePassive(context.Context, string, protocol.CapturePassiveInput) (protocol.CapturePassiveOutput, error)
	Context(context.Context, protocol.ContextBuildInput) (string, error)
	ListTags(context.Context, string) ([]protocol.TagInfo, error)
	TagStats(context.Context, protocol.TagStatsInput) ([]protocol.TagInfo, error)
	MergeTags(context.Context, string, protocol.TagMergeInput) (protocol.TagMergeOutput, error)
	RelatedTags(context.Context, protocol.TagRelatedInput) ([]protocol.RelatedTag, error)
	Stats(context.Context) (protocol.StatsOutput, error)
	Timeline(context.Context, protocol.ObservationTimelineInput) (protocol.ObservationTimelineOutput, error)
	Get(context.Context, protocol.ResourceRef) (protocol.Observation, error)
	Limits(context.Context) (protocol.LimitsOutput, error)
}

// ControllerBackend is the Mnemo public client used by the MCP presentation
// adapter; it introduces no new transport, proxy or store implementation.
type ControllerBackend struct{ *client.Client }

func NewControllerBackend(cfg events.Config) (*ControllerBackend, error) {
	api, err := client.New(filepath.Dir(cfg.DataDir))
	if err != nil { return nil, err }
	return &ControllerBackend{Client: api}, nil
}

var _ MemoryBackend = (*ControllerBackend)(nil)
