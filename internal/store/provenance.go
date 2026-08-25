package store

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"unicode"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

const (
	AgentUnknown    = "unknown"
	AgentExternal   = "external"
	AgentCLI        = "cli"
	AgentCodex      = "codex"
	AgentClaudeCode = "claudecode"
	AgentCursor     = "cursor"
	AgentWindsurf   = "windsurf"
	AgentOpenCode   = "opencode"
	AgentFx         = "fx"
	AgentPi         = "pi"

	SourceUnknown        = "unknown"
	SourceCLI            = "cli"
	SourceMCP            = "mcp"
	SourceHook           = "hook"
	SourcePassiveCapture = "passive_capture"
	SourceImport         = "import"
	SourceSkill          = "skill"
	SourceSync           = "sync"

	ToolUnknown           = "unknown"
	ToolMnemoSave         = "mnemo_save"
	ToolMemSave           = "mem_save"
	ToolMemSavePrompt     = "mem_save_prompt"
	ToolMemSessionStart   = "mem_session_start"
	ToolMemSessionEnd     = "mem_session_end"
	ToolMemSessionSummary = "mem_session_summary"
	ToolMemCapturePassive = "mem_capture_passive"
	ToolMnemoCapture      = "mnemo_capture"
	ToolMnemoImport       = "mnemo_import"
	ToolSyncPull          = "sync_pull"
	ToolHookSessionStart  = "hook_session_start"
	ToolHookSessionStop   = "hook_session_stop"

	ModelUnknown     = "unknown"
	MCPClientNone    = "none"
	MCPClientUnknown = "unknown"
)

func CLIProvenance(tool string) ProvenanceInput {
	agent := normalizeCatalogID(firstNonEmpty(getenv("MNEMO_AGENT"), AgentCLI), AgentCLI)
	source := normalizeCatalogID(firstNonEmpty(getenv("MNEMO_SOURCE"), SourceCLI), SourceCLI)
	return ProvenanceInput{
		AgentID:       agent,
		SourceKindID:  source,
		ToolID:        tool,
		ModelID:       getenv("MNEMO_MODEL"),
		ModelProvider: getenv("MNEMO_MODEL_PROVIDER"),
	}
}

func MCPProvenance(tool string) ProvenanceInput {
	agent := normalizeCatalogID(firstNonEmpty(getenv("MNEMO_AGENT"), AgentUnknown), AgentUnknown)
	clientID := normalizeCatalogID(firstNonEmpty(getenv("MNEMO_MCP_CLIENT"), agent), MCPClientUnknown)
	return ProvenanceInput{
		AgentID:          agent,
		SourceKindID:     SourceMCP,
		ToolID:           tool,
		ModelID:          getenv("MNEMO_MODEL"),
		ModelProvider:    getenv("MNEMO_MODEL_PROVIDER"),
		MCPClientID:      clientID,
		MCPClientName:    firstNonEmpty(getenv("MNEMO_MCP_CLIENT_NAME"), displayName(clientID)),
		MCPClientVersion: getenv("MNEMO_MCP_CLIENT_VERSION"),
		MCPTransport:     firstNonEmpty(getenv("MNEMO_MCP_TRANSPORT"), "stdio"),
	}
}

func HookProvenance(agent, tool string) ProvenanceInput {
	return ProvenanceInput{AgentID: agent, SourceKindID: SourceHook, ToolID: tool}
}

func SyncPullProvenance() ProvenanceInput {
	return ProvenanceInput{AgentID: AgentExternal, SourceKindID: SourceSync, ToolID: ToolSyncPull}
}

func (s *Store) seedProvenanceCatalog() error {
	statements := []string{
		`INSERT OR IGNORE INTO agents (id, display_name, kind) VALUES
			('unknown', 'Unknown', 'unknown'),
			('external', 'External', 'agent'),
			('cli', 'CLI', 'cli'),
			('codex', 'Codex', 'agent'),
			('claudecode', 'Claude Code', 'agent'),
			('cursor', 'Cursor', 'agent'),
			('windsurf', 'Windsurf', 'agent'),
			('opencode', 'OpenCode', 'agent'),
			('fx', 'fx', 'agent'),
			('pi', 'Pi', 'agent')`,
		`INSERT OR IGNORE INTO source_kinds (id, display_name) VALUES
			('unknown', 'Unknown'),
			('cli', 'CLI'),
			('mcp', 'MCP'),
			('hook', 'Hook'),
			('passive_capture', 'Passive Capture'),
			('import', 'Import'),
			('skill', 'Skill'),
			('sync', 'Sync')`,
		`INSERT OR IGNORE INTO tools (id, display_name) VALUES
			('unknown', 'Unknown'),
			('mnemo_save', 'mnemo save'),
			('mem_save', 'mem_save'),
			('mem_save_prompt', 'mem_save_prompt'),
			('mem_session_start', 'mem_session_start'),
			('mem_session_end', 'mem_session_end'),
			('mem_session_summary', 'mem_session_summary'),
			('mem_capture_passive', 'mem_capture_passive'),
			('mnemo_capture', 'mnemo capture'),
			('mnemo_import', 'mnemo import'),
			('sync_pull', 'sync pull'),
			('hook_session_start', 'hook session start'),
			('hook_session_stop', 'hook session stop')`,
		`INSERT OR IGNORE INTO models (id, provider, display_name) VALUES
			('unknown', '', 'Unknown')`,
		`INSERT OR IGNORE INTO mcp_clients (id, name, version, transport) VALUES
			('none', 'None', '', ''),
			('unknown', 'Unknown', '', '')`,
	}
	for _, stmt := range statements {
		if _, err := s.execHook(s.db, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureProvenanceTx(tx *sql.Tx, input ProvenanceInput) (int64, error) {
	q := s.q.WithTx(tx)
	normalized := normalizeProvenance(input)
	ctx := context.Background()

	if err := q.UpsertAgent(ctx, dbgen.UpsertAgentParams{
		ID: normalized.AgentID, DisplayName: displayName(normalized.AgentID), Kind: agentKind(normalized.AgentID),
	}); err != nil {
		return 0, err
	}
	if err := q.UpsertSourceKind(ctx, dbgen.UpsertSourceKindParams{
		ID: normalized.SourceKindID, DisplayName: displayName(normalized.SourceKindID),
	}); err != nil {
		return 0, err
	}
	if err := q.UpsertTool(ctx, dbgen.UpsertToolParams{
		ID: normalized.ToolID, DisplayName: displayName(normalized.ToolID),
	}); err != nil {
		return 0, err
	}
	if err := q.UpsertModel(ctx, dbgen.UpsertModelParams{
		ID: normalized.ModelID, Provider: normalized.ModelProvider, DisplayName: displayName(normalized.ModelID),
	}); err != nil {
		return 0, err
	}
	if err := q.UpsertMCPClient(ctx, dbgen.UpsertMCPClientParams{
		ID: normalized.MCPClientID, Name: normalized.MCPClientName,
		Version: normalized.MCPClientVersion, Transport: normalized.MCPTransport,
	}); err != nil {
		return 0, err
	}

	params := dbgen.InsertProvenanceContextParams{
		AgentID: normalized.AgentID, SourceKindID: normalized.SourceKindID, ToolID: normalized.ToolID,
		ModelID: normalized.ModelID, McpClientID: normalized.MCPClientID,
	}
	id, err := q.InsertProvenanceContext(ctx, params)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	return q.GetProvenanceContextID(ctx, dbgen.GetProvenanceContextIDParams(params))
}

func (s *Store) optionalProvenanceTx(tx *sql.Tx, input ProvenanceInput) (sql.NullInt64, error) {
	if !hasProvenanceInput(input) {
		return sql.NullInt64{}, nil
	}
	id, err := s.ensureProvenanceTx(tx, input)
	if err != nil {
		return sql.NullInt64{}, err
	}
	return sqlNullInt64(id), nil
}

func (s *Store) getProvenance(id int64) (*Provenance, error) {
	row, err := s.q.GetProvenanceContext(context.Background(), id)
	if err != nil {
		return nil, err
	}
	provenance := provenanceFromDB(row)
	return &provenance, nil
}

func getProvenanceTx(q *dbgen.Queries, id int64) (*Provenance, error) {
	row, err := q.GetProvenanceContext(context.Background(), id)
	if err != nil {
		return nil, err
	}
	provenance := provenanceFromDB(row)
	return &provenance, nil
}

func (s *Store) attachObservationProvenance(obs *Observation, provenanceID sql.NullInt64) error {
	if obs == nil || !provenanceID.Valid {
		return nil
	}
	provenance, err := s.getProvenance(provenanceID.Int64)
	if err != nil {
		return err
	}
	obs.Provenance = provenance
	return nil
}

func (s *Store) attachObservationsProvenance(observations []Observation, provenanceIDs []sql.NullInt64) error {
	for i := range observations {
		if i >= len(provenanceIDs) {
			break
		}
		if err := s.attachObservationProvenance(&observations[i], provenanceIDs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) attachSearchResultsProvenance(results []SearchResult, provenanceIDs []sql.NullInt64) error {
	for i := range results {
		if i >= len(provenanceIDs) {
			break
		}
		if err := s.attachObservationProvenance(&results[i].Observation, provenanceIDs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) attachPromptProvenance(prompt *Prompt, provenanceID sql.NullInt64) error {
	if prompt == nil || !provenanceID.Valid {
		return nil
	}
	provenance, err := s.getProvenance(provenanceID.Int64)
	if err != nil {
		return err
	}
	prompt.Provenance = provenance
	return nil
}

func (s *Store) attachPromptsProvenance(prompts []Prompt, provenanceIDs []sql.NullInt64) error {
	for i := range prompts {
		if i >= len(provenanceIDs) {
			break
		}
		if err := s.attachPromptProvenance(&prompts[i], provenanceIDs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) attachSessionSummariesProvenance(summaries []SessionSummary, provenanceIDs []sql.NullInt64) error {
	for i := range summaries {
		if i >= len(provenanceIDs) {
			break
		}
		if !provenanceIDs[i].Valid {
			continue
		}
		provenance, err := s.getProvenance(provenanceIDs[i].Int64)
		if err != nil {
			return err
		}
		summaries[i].Provenance = provenance
	}
	return nil
}

func (s *Store) attachTimelineEntryProvenance(entry *TimelineEntry, provenanceID sql.NullInt64) error {
	if entry == nil || !provenanceID.Valid {
		return nil
	}
	provenance, err := s.getProvenance(provenanceID.Int64)
	if err != nil {
		return err
	}
	entry.Provenance = provenance
	return nil
}

func attachObservationProvenanceTx(q *dbgen.Queries, obs *Observation, provenanceID sql.NullInt64) error {
	if obs == nil || !provenanceID.Valid {
		return nil
	}
	provenance, err := getProvenanceTx(q, provenanceID.Int64)
	if err != nil {
		return err
	}
	obs.Provenance = provenance
	return nil
}

func nullableProvenanceInput(input ProvenanceInput) *ProvenanceInput {
	if !hasProvenanceInput(input) {
		return nil
	}
	normalized := normalizeProvenance(input)
	return &normalized
}

func provenanceInputFromPtr(input *ProvenanceInput) ProvenanceInput {
	if input == nil {
		return ProvenanceInput{}
	}
	return *input
}

func provenanceInputForID(q *dbgen.Queries, provenanceID sql.NullInt64) *ProvenanceInput {
	if !provenanceID.Valid {
		return nil
	}
	row, err := q.GetProvenanceContext(context.Background(), provenanceID.Int64)
	if err != nil {
		return nil
	}
	provenance := provenanceFromDB(row)
	return nullableProvenanceInput(provenanceInputFromStored(&provenance, ProvenanceInput{}))
}

func hasProvenanceInput(input ProvenanceInput) bool {
	return strings.TrimSpace(input.AgentID) != "" ||
		strings.TrimSpace(input.SourceKindID) != "" ||
		strings.TrimSpace(input.ToolID) != "" ||
		strings.TrimSpace(input.ModelID) != "" ||
		strings.TrimSpace(input.ModelProvider) != "" ||
		strings.TrimSpace(input.MCPClientID) != "" ||
		strings.TrimSpace(input.MCPClientName) != "" ||
		strings.TrimSpace(input.MCPClientVersion) != "" ||
		strings.TrimSpace(input.MCPTransport) != ""
}

func normalizeProvenance(input ProvenanceInput) ProvenanceInput {
	agentID := normalizeCatalogID(input.AgentID, AgentUnknown)
	sourceID := normalizeCatalogID(input.SourceKindID, SourceUnknown)
	toolID := normalizeCatalogID(input.ToolID, ToolUnknown)
	modelID := normalizeCatalogID(input.ModelID, ModelUnknown)
	mcpClientID := normalizeCatalogID(input.MCPClientID, MCPClientNone)
	if sourceID == SourceMCP && mcpClientID == MCPClientNone {
		mcpClientID = MCPClientUnknown
	}
	mcpName := strings.TrimSpace(input.MCPClientName)
	if mcpName == "" {
		mcpName = displayName(mcpClientID)
	}
	return ProvenanceInput{
		AgentID:          agentID,
		SourceKindID:     sourceID,
		ToolID:           toolID,
		ModelID:          modelID,
		ModelProvider:    strings.TrimSpace(input.ModelProvider),
		MCPClientID:      mcpClientID,
		MCPClientName:    mcpName,
		MCPClientVersion: strings.TrimSpace(input.MCPClientVersion),
		MCPTransport:     strings.TrimSpace(input.MCPTransport),
	}
}

func normalizeCatalogID(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fallback
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == '/' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	normalized := strings.Trim(b.String(), "-")
	if normalized == "" {
		return fallback
	}
	return normalized
}

func displayName(id string) string {
	names := map[string]string{
		"unknown":         "Unknown",
		"none":            "None",
		"cli":             "CLI",
		"codex":           "Codex",
		"claudecode":      "Claude Code",
		"cursor":          "Cursor",
		"windsurf":        "Windsurf",
		"opencode":        "OpenCode",
		"fx":              "fx",
		"pi":              "Pi",
		"mcp":             "MCP",
		"hook":            "Hook",
		"passive_capture": "Passive Capture",
		"import":          "Import",
		"skill":           "Skill",
		"sync":            "Sync",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return strings.ReplaceAll(id, "_", " ")
}

func agentKind(agentID string) string {
	if agentID == AgentCLI {
		return "cli"
	}
	if agentID == AgentUnknown {
		return "unknown"
	}
	return "agent"
}

var getenv = func(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
