package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmeiracorbal/mnemo/adapters"
)

// ApplyMCPCommand executes one durable MCP mutation and stores its serialized
// result in the same SQLite transaction. A JetStream redelivery therefore
// receives the original result without applying the mutation again.
func (s *Store) ApplyMCPCommand(id, action string, payload json.RawMessage) (json.RawMessage, error) {
	var result json.RawMessage
	err := s.withTx(func(tx *sql.Tx) error {
		var prior string
		err := tx.QueryRow(`SELECT result_json FROM processed_mcp_commands WHERE id = ?`, id).Scan(&prior)
		if err == nil {
			result = json.RawMessage(prior)
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		transactional := *s
		transactional.activeTx = tx
		transactional.q = s.q.WithTx(tx)
		computed, err := transactional.ExecuteMCPAction(context.Background(), action, payload)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO processed_mcp_commands (id, action, result_json) VALUES (?, ?, ?)`, id, action, string(computed)); err != nil {
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
			Agent     adapters.Agent `json:"agent"`
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
	default:
		return nil, fmt.Errorf("unsupported MCP action %q", action)
	}
	return json.Marshal(result)
}
