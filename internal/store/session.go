package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func (s *Store) CreateSession(id, project, directory string) error {
	return s.withTx(func(tx *sql.Tx) error {
		if err := s.createSessionTx(tx, id, project, directory); err != nil {
			return err
		}
		return s.enqueueSyncMutationTx(tx, SyncEntitySession, id, SyncOpUpsert, syncSessionPayload{
			ID:        id,
			Project:   project,
			Directory: directory,
		})
	})
}

func (s *Store) EndSession(id string, summary string) error {
	return s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.GetSessionPayload(context.Background(), id); err == sql.ErrNoRows {
			return nil
		} else if err != nil {
			return err
		}
		if err := q.EndSession(context.Background(), dbgen.EndSessionParams{
			Summary: sqlNullString(summary), ID: id,
		}); err != nil {
			return err
		}
		stored, err := q.GetSessionPayload(context.Background(), id)
		if err != nil {
			return err
		}
		endedAt := nullablePtr(stored.EndedAt)
		storedSummary := nullablePtr(stored.Summary)

		return s.enqueueSyncMutationTx(tx, SyncEntitySession, id, SyncOpUpsert, syncSessionPayload{
			ID:        id,
			Project:   stored.Project,
			Directory: stored.Directory,
			EndedAt:   endedAt,
			Summary:   storedSummary,
		})
	})
}

func (s *Store) GetSession(id string) (*Session, error) {
	row, err := s.q.GetSession(context.Background(), id)
	if err != nil {
		return nil, err
	}
	sess := Session{
		ID: row.ID, Project: row.Project, Directory: row.Directory, StartedAt: row.StartedAt,
		EndedAt: nullablePtr(row.EndedAt), Summary: nullablePtr(row.Summary),
	}
	if err := s.loadTagsForSession(&sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// ObsCount returns the number of non-deleted observations for a given session.
func (s *Store) ObsCount(sessionID string) (int, error) {
	count, err := s.q.CountSessionObservations(context.Background(), sessionID)
	return int(count), err
}

// ObsCountForSession counts all non-deleted observations saved for the session's
// project on or after the session started.
func (s *Store) ObsCountForSession(sessionID string) (int, error) {
	count, err := s.q.CountObservationsForSessionProject(context.Background(), sessionID)
	return int(count), err
}

func (s *Store) RecentSessions(project string, limit int) ([]SessionSummary, error) {
	if limit <= 0 {
		limit = 5
	}
	return s.listSessions(project, limit)
}

func (s *Store) AllSessions(project string, limit int) ([]SessionSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.listSessions(project, limit)
}

func (s *Store) listSessions(project string, limit int) ([]SessionSummary, error) {
	rows, err := s.q.ListSessions(context.Background(), dbgen.ListSessionsParams{
		Project: project, ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]SessionSummary, 0, len(rows))
	for _, row := range rows {
		results = append(results, SessionSummary{
			ID: row.ID, Project: row.Project, StartedAt: row.StartedAt,
			EndedAt: nullablePtr(row.EndedAt), Summary: nullablePtr(row.Summary),
			ObservationCount: int(row.ObservationCount),
		})
	}
	return results, nil
}

func (s *Store) SessionObservations(sessionID string, limit int) ([]Observation, error) {
	if limit <= 0 {
		limit = 200
	}

	rows, err := s.q.ListSessionObservations(context.Background(), dbgen.ListSessionObservationsParams{
		SessionID: sessionID, ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]Observation, 0, len(rows))
	for _, row := range rows {
		results = append(results, observationFromSessionRow(row))
	}
	if err := s.loadTagsForObservations(results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) Stats() (*Stats, error) {
	stats := &Stats{}
	ctx := context.Background()
	if value, err := s.q.CountSessions(ctx); err == nil {
		stats.TotalSessions = int(value)
	}
	if value, err := s.q.CountLiveObservations(ctx); err == nil {
		stats.TotalObservations = int(value)
	}
	if value, err := s.q.CountPrompts(ctx); err == nil {
		stats.TotalPrompts = int(value)
	}
	if projects, err := s.q.ListObservationProjects(ctx); err == nil {
		for _, project := range projects {
			if project.Valid {
				stats.Projects = append(stats.Projects, project.String)
			}
		}
	}
	return stats, nil
}

// FormatContext returns a formatted context string for agent consumption.
func (s *Store) FormatContext(project, scope string, tags ...string) (string, error) {
	return s.FormatContextOpts(project, scope, ContextOptions{Tags: tags})
}

// FormatContextOpts is like FormatContext but accepts full ContextOptions.
func (s *Store) FormatContextOpts(project, scope string, opts ContextOptions) (string, error) {
	sessions, err := s.RecentSessions(project, 5)
	if err != nil {
		return "", err
	}

	observations, err := s.recentObservations(project, scope, s.cfg.MaxContextResults, opts)
	if err != nil {
		return "", err
	}

	prompts, err := s.RecentPrompts(project, 10)
	if err != nil {
		return "", err
	}

	if len(sessions) == 0 && len(observations) == 0 && len(prompts) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("## Memory from Previous Sessions\n\n")

	if len(sessions) > 0 {
		b.WriteString("### Recent Sessions\n")
		for _, sess := range sessions {
			summary := ""
			if sess.Summary != nil {
				summary = fmt.Sprintf(": %s", truncate(*sess.Summary, 200))
			}
			fmt.Fprintf(&b, "- **%s** (%s)%s [%d observations]\n",
				sess.Project, sess.StartedAt, summary, sess.ObservationCount)
		}
		b.WriteString("\n")
	}

	if len(prompts) > 0 {
		b.WriteString("### Recent User Prompts\n")
		for _, p := range prompts {
			fmt.Fprintf(&b, "- %s: %s\n", p.CreatedAt, truncate(p.Content, 200))
		}
		b.WriteString("\n")
	}

	if len(observations) > 0 {
		b.WriteString("### Recent Observations\n")
		for _, obs := range observations {
			fmt.Fprintf(&b, "- [%s] **%s**: %s\n",
				obs.Type, obs.Title, truncate(obs.Content, 300))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func (s *Store) createSessionTx(tx *sql.Tx, id, project, directory string) error {
	return s.q.WithTx(tx).UpsertSession(context.Background(), dbgen.UpsertSessionParams{
		ID: id, Project: project, Directory: directory,
	})
}
