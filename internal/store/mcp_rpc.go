package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmeiracorbal/mnemo-adapters/agents"
	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

// ApplyMCPCommand executes one durable MCP mutation and stores its serialized
// result in the same SQLite transaction. A JetStream redelivery therefore
// receives the original result without applying the mutation again.
func (s *Store) ApplyMCPCommand(id, action string, payload json.RawMessage) (json.RawMessage, error) {
	var result json.RawMessage
	err := s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		prior, err := q.GetProcessedMCPCommandResult(context.Background(), id)
		if err == nil {
			result = json.RawMessage(prior)
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		transactional := *s
		transactional.activeTx = tx
		transactional.q = q
		computed, err := transactional.ExecuteMCPAction(context.Background(), action, payload)
		if err != nil {
			return err
		}
		if err := q.InsertProcessedMCPCommand(context.Background(), dbgen.InsertProcessedMCPCommandParams{
			ID: id, Action: action, ResultJson: string(computed),
		}); err != nil {
			return err
		}
		result = computed
		return nil
	})
	return result, err
}

// ExecuteMCPAction is the controller-only RPC surface for MCP operations.
// Keeping this dispatch next to Store makes the controller the only process
// that can translate an MCP request into SQLite access.
func (s *Store) ExecuteMCPAction(ctx context.Context, action string, payload json.RawMessage) (json.RawMessage, error) {
	_ = ctx // Store's public operations use their own transaction contexts.
	var result any
	switch action {
	case "search":
		var input struct {
			Query   string        `json:"query"`
			Options SearchOptions `json:"options"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.Search(input.Query, input.Options)
		if err != nil {
			return nil, err
		}
		result = value
	case "resolve_session":
		var input struct {
			Project    string `json:"project"`
			Directory  string `json:"directory"`
			InstanceID string `json:"instance_id"`
			PID        int    `json:"pid"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ResolveMCPInstanceSession(input.Project, input.Directory, input.InstanceID, input.PID)
		if err != nil {
			return nil, err
		}
		result = value
	case "close_sessions":
		var input struct {
			InstanceID string `json:"instance_id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		if err := s.CloseMCPInstanceSessions(input.InstanceID); err != nil {
			return nil, err
		}
	case "bind_execution_session":
		var input struct {
			Project   string         `json:"project"`
			Agent     agents.Agent `json:"agent"`
			NativeID  string         `json:"native_id"`
			SessionID string         `json:"session_id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		if err := s.BindExecutionSession(input.Project, input.Agent, input.NativeID, input.SessionID); err != nil {
			return nil, err
		}
	case "add_observation":
		var input AddObservationParams
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.AddObservation(input)
		if err != nil {
			return nil, err
		}
		result = value
	case "update_observation":
		var input struct {
			ID     int64                   `json:"id"`
			Params UpdateObservationParams `json:"params"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.UpdateObservation(input.ID, input.Params)
		if err != nil {
			return nil, err
		}
		result = value
	case "delete_observation":
		var input struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		if err := s.DeleteObservation(input.ID); err != nil {
			return nil, err
		}
	case "add_prompt":
		var input AddPromptParams
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.AddPrompt(input)
		if err != nil {
			return nil, err
		}
		result = value
	case "passive_capture":
		var input PassiveCaptureParams
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.PassiveCapture(input)
		if err != nil {
			return nil, err
		}
		result = value
	case "format_context":
		var input struct {
			Project string         `json:"project"`
			Scope   string         `json:"scope"`
			Options ContextOptions `json:"options"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.FormatContextOpts(input.Project, input.Scope, input.Options)
		if err != nil {
			return nil, err
		}
		result = value
	case "list_tags":
		var input struct {
			Project string `json:"project"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ListTags(input.Project)
		if err != nil {
			return nil, err
		}
		result = value
	case "tag_stats":
		var input struct {
			Project string          `json:"project"`
			Options TagStatsOptions `json:"options"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.TagStats(input.Project, input.Options)
		if err != nil {
			return nil, err
		}
		result = value
	case "merge_tags":
		var input struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		observations, sessions, err := s.MergeTags(input.From, input.To)
		if err != nil {
			return nil, err
		}
		result = struct {
			Observations int `json:"observations"`
			Sessions     int `json:"sessions"`
		}{observations, sessions}
	case "related_tags":
		var input struct {
			Project string             `json:"project"`
			Tag     string             `json:"tag"`
			Options RelatedTagsOptions `json:"options"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.RelatedTags(input.Project, input.Tag, input.Options)
		if err != nil {
			return nil, err
		}
		result = value
	case "stats":
		value, err := s.Stats()
		if err != nil {
			return nil, err
		}
		result = value
	case "timeline":
		var input struct {
			ObservationID int64 `json:"observation_id"`
			Before        int   `json:"before"`
			After         int   `json:"after"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.Timeline(input.ObservationID, input.Before, input.After)
		if err != nil {
			return nil, err
		}
		result = value
	case "get_observation":
		var input struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.GetObservation(input.ID)
		if err != nil {
			return nil, err
		}
		result = value
	case "max_observation_length":
		result = s.MaxObservationLength()
	case "agent_tool":
		var input AgentToolRequest
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ExecuteAgentTool(input)
		if err != nil {
			return nil, err
		}
		result = value
	case "export":
		value, err := s.Export()
		if err != nil {
			return nil, err
		}
		result = value
	case "import":
		var input ExportData
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.Import(&input)
		if err != nil {
			return nil, err
		}
		result = value
	case "list_project_summaries":
		value, err := s.ListProjectSummaries()
		if err != nil {
			return nil, err
		}
		result = value
	case "build_project_merge_plan":
		var input struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.BuildProjectMergePlan(input.From, input.To)
		if err != nil {
			return nil, err
		}
		result = value
	case "merge_projects":
		var input struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.MergeProjects(input.From, input.To)
		if err != nil {
			return nil, err
		}
		result = value
	case "build_project_rename_plan":
		var input struct {
			Selector ProjectRenameSelector `json:"selector"`
			Name     string                `json:"name"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.BuildProjectRenamePlan(input.Selector, input.Name)
		if err != nil {
			return nil, err
		}
		result = value
	case "rename_project":
		var input struct {
			Selector ProjectRenameSelector `json:"selector"`
			Name     string                `json:"name"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.RenameProject(input.Selector, input.Name)
		if err != nil {
			return nil, err
		}
		result = value
	case "session_exists":
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		_, err := s.GetSession(input.ID)
		result = err == nil
	case "session_obs_count":
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ObsCount(input.ID)
		if err != nil {
			return nil, err
		}
		result = value
	case "session_project_obs_count":
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ObsCountForSession(input.ID)
		if err != nil {
			return nil, err
		}
		result = value
	case "review_memories":
		var input MemoryReviewOptions
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ReviewMemoryConflicts(input)
		if err != nil {
			return nil, err
		}
		result = value
	case "mark_memory_reviewed":
		var input struct {
			ID     int64  `json:"id"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		if err := s.MarkMemoryReviewed(input.ID, input.Reason); err != nil {
			return nil, err
		}
	case "mark_memory_stale":
		var input struct {
			ID     int64  `json:"id"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		if err := s.MarkMemoryStale(input.ID, input.Reason); err != nil {
			return nil, err
		}
	case "supersede_memory":
		var input struct {
			OldID  int64  `json:"old_id"`
			NewID  int64  `json:"new_id"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		if err := s.SupersedeMemory(input.OldID, input.NewID, input.Reason); err != nil {
			return nil, err
		}
	case "plan_consolidate_topic":
		var input struct {
			FromTopic string `json:"from_topic"`
			ToTopic   string `json:"to_topic"`
			Project   string `json:"project"`
			Scope     string `json:"scope"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.PlanMemoryTopicConsolidation(input.FromTopic, input.ToTopic, input.Project, input.Scope)
		if err != nil {
			return nil, err
		}
		result = value
	case "consolidate_topic":
		var input struct {
			FromTopic string `json:"from_topic"`
			ToTopic   string `json:"to_topic"`
			Project   string `json:"project"`
			Scope     string `json:"scope"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		value, err := s.ConsolidateMemoryTopic(input.FromTopic, input.ToTopic, input.Project, input.Scope)
		if err != nil {
			return nil, err
		}
		result = value
	case "migrate_project_identity":
		var input struct {
			LegacyKey string `json:"legacy_key"`
			ProjectID string `json:"project_id"`
			Name      string `json:"name"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, err
		}
		migrateResult, err := s.MigrateProject(input.LegacyKey, input.ProjectID)
		if err != nil {
			return nil, err
		}
		if err := s.EnsureProject(input.ProjectID, input.Name); err != nil {
			return nil, err
		}
		result = migrateResult
	default:
		return nil, fmt.Errorf("unsupported MCP action %q", action)
	}
	return json.Marshal(result)
}
