package store

import (
	"context"
	"database/sql"
	"fmt"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func (s *Store) EnsureProject(id, name string) error {
	return s.q.EnsureProject(context.Background(), dbgen.EnsureProjectParams{ID: id, Name: name})
}

func (s *Store) GetProjectByID(id string) (*Project, error) {
	p, err := s.q.GetProjectByID(context.Background(), id)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &Project{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt}, nil
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.q.ListProjects(context.Background())
	if err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(rows))
	for _, p := range rows {
		projects = append(projects, Project{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt})
	}
	return projects, nil
}

func (s *Store) MigrateProject(oldName, newName string) (*MigrateResult, error) {
	if oldName == "" || newName == "" || oldName == newName {
		return &MigrateResult{}, nil
	}

	exists, err := s.q.ProjectExists(context.Background(), sqlNullString(oldName))
	if err != nil {
		return nil, fmt.Errorf("check old project: %w", err)
	}
	if !exists {
		return &MigrateResult{}, nil
	}

	result := &MigrateResult{Migrated: true}

	err = s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		params := dbgen.RenameObservationProjectParams{NewName: sqlNullString(newName), OldName: sqlNullString(oldName)}
		n, err := q.RenameObservationProject(context.Background(), params)
		if err != nil {
			return fmt.Errorf("migrate observations: %w", err)
		}
		result.ObservationsUpdated = n

		result.SessionsUpdated, err = q.RenameSessionProject(context.Background(), dbgen.RenameSessionProjectParams{
			NewName: newName, OldName: oldName,
		})
		if err != nil {
			return fmt.Errorf("migrate sessions: %w", err)
		}

		result.PromptsUpdated, err = q.RenamePromptProject(context.Background(), dbgen.RenamePromptProjectParams{
			NewName: sqlNullString(newName), OldName: sqlNullString(oldName),
		})
		if err != nil {
			return fmt.Errorf("migrate prompts: %w", err)
		}

		result.SyncMutationsUpdated, err = q.RenameMutationProject(context.Background(), dbgen.RenameMutationProjectParams{
			NewName: newName, OldName: oldName,
		})
		if err != nil {
			return fmt.Errorf("migrate sync_mutations: %w", err)
		}

		if err = q.CopyProjectEnrollment(context.Background(), dbgen.CopyProjectEnrollmentParams{
			NewName: newName, OldName: oldName,
		}); err != nil {
			return fmt.Errorf("migrate sync_enrolled_projects insert: %w", err)
		}
		if err = q.DeleteProjectEnrollment(context.Background(), oldName); err != nil {
			return fmt.Errorf("migrate sync_enrolled_projects delete: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Store) EnrollProject(project string) error {
	if project == "" {
		return fmt.Errorf("project name must not be empty")
	}
	return s.withTx(func(tx *sql.Tx) error {
		rowsAffected, err := s.q.WithTx(tx).EnrollProject(context.Background(), project)
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return nil
		}
		return s.backfillProjectSyncMutationsTx(tx, project)
	})
}

func (s *Store) UnenrollProject(project string) error {
	if project == "" {
		return fmt.Errorf("project name must not be empty")
	}
	return s.q.UnenrollProject(context.Background(), project)
}

func (s *Store) ListEnrolledProjects() ([]EnrolledProject, error) {
	rows, err := s.q.ListEnrolledProjects(context.Background())
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	projects := make([]EnrolledProject, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, EnrolledProject{Project: row.Project, EnrolledAt: row.EnrolledAt})
	}
	return projects, nil
}

func (s *Store) IsProjectEnrolled(project string) (bool, error) {
	return s.q.IsProjectEnrolled(context.Background(), project)
}
