package store

import (
	"context"
	"database/sql"
	"fmt"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func (s *Store) Export() (*ExportData, error) {
	data := &ExportData{
		Version:    "0.1.0",
		ExportedAt: Now(),
	}

	rows, err := s.q.ExportSessions(context.Background())
	if err != nil {
		return nil, fmt.Errorf("export sessions: %w", err)
	}
	for _, row := range rows {
		sess := Session{
			ID: row.ID, Project: row.Project, Directory: row.Directory, StartedAt: row.StartedAt,
			EndedAt: nullablePtr(row.EndedAt), Summary: nullablePtr(row.Summary),
		}
		if err := s.loadTagsForSession(&sess); err != nil {
			return nil, fmt.Errorf("export sessions: load tags: %w", err)
		}
		data.Sessions = append(data.Sessions, sess)
	}

	obsRows, err := s.q.ExportObservations(context.Background())
	if err != nil {
		return nil, fmt.Errorf("export observations: %w", err)
	}
	for _, row := range obsRows {
		o := observationFromDB(row.ID, row.SyncID, row.SessionID, row.Type, row.Title, row.Content,
			row.ToolName, row.Project, row.Scope, row.TopicKey, row.RevisionCount, row.DuplicateCount,
			row.LastSeenAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
		data.Observations = append(data.Observations, o)
	}
	if err := s.loadTagsForObservations(data.Observations); err != nil {
		return nil, fmt.Errorf("export observations: load tags: %w", err)
	}

	promptRows, err := s.q.ExportPrompts(context.Background())
	if err != nil {
		return nil, fmt.Errorf("export prompts: %w", err)
	}
	for _, row := range promptRows {
		data.Prompts = append(data.Prompts, Prompt{
			ID: row.ID, SyncID: dbString(row.SyncID), SessionID: row.SessionID,
			Content: row.Content, Project: dbString(row.Project), CreatedAt: row.CreatedAt,
		})
	}

	return data, nil
}

func (s *Store) Import(data *ExportData) (*ImportResult, error) {
	if data == nil {
		return nil, fmt.Errorf("import: nil export data")
	}
	tx, err := s.beginTxHook()
	if err != nil {
		return nil, fmt.Errorf("import: begin tx: %w", err)
	}
	defer tx.Rollback()

	result := &ImportResult{}
	q := s.q.WithTx(tx)

	for _, sess := range data.Sessions {
		n, err := q.ImportSession(context.Background(), dbgen.ImportSessionParams{
			ID: sess.ID, Project: sess.Project, Directory: sess.Directory, StartedAt: sess.StartedAt,
			EndedAt: sqlNullStringPtr(sess.EndedAt), Summary: sqlNullStringPtr(sess.Summary),
		})
		if err != nil {
			return nil, fmt.Errorf("import session %s: %w", sess.ID, err)
		}
		if n > 0 && sess.Tags != nil {
			if err := s.setTagsForSessionTx(tx, sess.ID, sess.Tags); err != nil {
				return nil, fmt.Errorf("import session %s: tags: %w", sess.ID, err)
			}
		}
		result.SessionsImported += int(n)
	}

	for _, obs := range data.Observations {
		if obs.SyncID != "" {
			_, err := q.GetObservationBySyncIDIncludingDeleted(context.Background(), sqlNullString(obs.SyncID))
			if err == nil {
				continue
			}
			if err != sql.ErrNoRows {
				return nil, fmt.Errorf("import observation check %d: %w", obs.ID, err)
			}
		} else {
			count, err := q.CountObservationsByHash(context.Background(), sqlNullString(hashNormalized(obs.Content)))
			if err != nil {
				return nil, fmt.Errorf("import observation check %d: %w", obs.ID, err)
			}
			if count > 0 {
				continue
			}
		}
		hash := hashNormalized(obs.Content)
		newID, err := q.ImportObservation(context.Background(), dbgen.ImportObservationParams{
			SyncID:    sqlNullString(normalizeExistingSyncID(obs.SyncID, "obs")),
			SessionID: obs.SessionID, Type: obs.Type, Title: obs.Title, Content: obs.Content,
			ToolName: sqlNullStringPtr(obs.ToolName), Project: sqlNullStringPtr(obs.Project),
			Scope: normalizeScope(obs.Scope), TopicKey: sqlNullString(normalizeTopicKey(derefString(obs.TopicKey))),
			NormalizedHash: sqlNullString(hash), RevisionCount: int64(maxInt(obs.RevisionCount, 1)),
			DuplicateCount: int64(maxInt(obs.DuplicateCount, 1)), LastSeenAt: sqlNullStringPtr(obs.LastSeenAt),
			CreatedAt: obs.CreatedAt, UpdatedAt: obs.UpdatedAt, DeletedAt: sqlNullStringPtr(obs.DeletedAt),
		})
		if err != nil {
			return nil, fmt.Errorf("import observation %d: %w", obs.ID, err)
		}
		if obs.Tags != nil {
			if err := s.setTagsForObservationTx(tx, newID, obs.Tags); err != nil {
				return nil, fmt.Errorf("import observation %d: tags: %w", obs.ID, err)
			}
		}
		result.ObservationsImported++
	}

	for _, p := range data.Prompts {
		err := q.ImportPrompt(context.Background(), dbgen.ImportPromptParams{
			SyncID:    sqlNullString(normalizeExistingSyncID(p.SyncID, "prompt")),
			SessionID: p.SessionID, Content: p.Content, Project: sqlNullString(p.Project), CreatedAt: p.CreatedAt,
		})
		if err != nil {
			return nil, fmt.Errorf("import prompt %d: %w", p.ID, err)
		}
		result.PromptsImported++
	}

	if err := s.commitHook(tx); err != nil {
		return nil, fmt.Errorf("import: commit: %w", err)
	}

	return result, nil
}

func (s *Store) GetSyncedChunks() (map[string]bool, error) {
	rows, err := s.q.ListSyncedChunks(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get synced chunks: %w", err)
	}
	chunks := make(map[string]bool)
	for _, id := range rows {
		chunks[id] = true
	}
	return chunks, nil
}

func (s *Store) RecordSyncedChunk(chunkID string) error {
	return s.q.InsertSyncedChunk(context.Background(), chunkID)
}
