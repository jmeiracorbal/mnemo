package protocol

// Archive DTOs are portable and never expose SQLite primary keys.
// Export/import share the Mnemo protocol revision, not the binary release.

type ArchiveSession struct {
	SessionID string `json:"session_id"`
	ProjectID string `json:"project_id"`
	Directory string `json:"directory"`
	StartedAt string `json:"started_at"`
	EndedAt *string `json:"ended_at,omitempty"`
	Summary *string `json:"summary,omitempty"`
	IsDeleted bool `json:"is_deleted"`
	Tags []string `json:"tags,omitempty"`
	Provenance *Provenance `json:"provenance,omitempty"`
}
type ArchiveObservation struct {
	Ref ResourceRef `json:"ref"`
	SessionID string `json:"session_id"`
	Type string `json:"type"`
	Title string `json:"title"`
	Content string `json:"content"`
	ToolName *string `json:"tool_name,omitempty"`
	Scope string `json:"scope"`
	TopicKey *string `json:"topic_key,omitempty"`
	Tags []string `json:"tags,omitempty"`
	RevisionCount int `json:"revision_count"`
	DuplicateCount int `json:"duplicate_count"`
	LastSeenAt *string `json:"last_seen_at,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	IsDeleted bool `json:"is_deleted"`
	Provenance *Provenance `json:"provenance,omitempty"`
}
type ArchivePrompt struct {
	ProjectID string `json:"project_id"`
	SyncID string `json:"sync_id"`
	SessionID string `json:"session_id"`
	Content string `json:"content"`
	CreatedAt string `json:"created_at"`
	IsDeleted bool `json:"is_deleted"`
	Provenance *Provenance `json:"provenance,omitempty"`
}
type ArchiveExportOutput struct {
	Version string `json:"version"`
	ExportedAt string `json:"exported_at"`
	Sessions []ArchiveSession `json:"sessions"`
	Observations []ArchiveObservation `json:"observations"`
	Prompts []ArchivePrompt `json:"prompts"`
}
type ArchiveImportInput struct {
	Archive ArchiveExportOutput `json:"archive"`
}
type ArchiveImportOutput struct {
	SessionsImported int `json:"sessions_imported"`
	ObservationsImported int `json:"observations_imported"`
	PromptsImported int `json:"prompts_imported"`
}