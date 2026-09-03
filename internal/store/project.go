package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

func (s *Store) EnsureProject(id, name string) error {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		return fmt.Errorf("project id must not be empty")
	}
	if name == "" {
		name = id
	}
	return s.q.EnsureProject(context.Background(), dbgen.EnsureProjectParams{ID: id, Name: name})
}

func (s *Store) ensureProjectTx(tx *sql.Tx, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	return s.q.WithTx(tx).EnsureProject(context.Background(), dbgen.EnsureProjectParams{ID: id, Name: id})
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

func (s *Store) ListProjectSummaries() ([]ProjectSummary, error) {
	rows, err := s.q.ListProjectSummaries(context.Background())
	if err != nil {
		return nil, err
	}
	projects := make([]ProjectSummary, 0, len(rows))
	for _, p := range rows {
		projects = append(projects, ProjectSummary{
			ID:               p.ID,
			Name:             p.Name,
			CreatedAt:        p.CreatedAt,
			Directory:        p.Directory,
			SessionCount:     int(p.SessionCount),
			ObservationCount: int(p.ObservationCount),
			PromptCount:      int(p.PromptCount),
			LastSeenAt:       p.LastSeenAt,
		})
	}
	return projects, nil
}

func (s *Store) BuildProjectMergePlan(from, to string) (*ProjectMergePlan, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" {
		return nil, fmt.Errorf("source project must not be empty")
	}
	if to == "" {
		return nil, fmt.Errorf("destination project must not be empty")
	}
	if from == to {
		return nil, fmt.Errorf("source and destination projects must differ")
	}

	projects, err := s.ListProjectSummaries()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]ProjectSummary, len(projects))
	for _, project := range projects {
		byID[project.ID] = project
	}
	source, ok := byID[from]
	if !ok {
		return nil, fmt.Errorf("source project %q not found", from)
	}
	destination, ok := byID[to]
	if !ok {
		return nil, fmt.Errorf("destination project %q not found", to)
	}

	q := s.q
	ctx := context.Background()
	observations, err := q.CountObservationProjectRows(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("count source observations: %w", err)
	}
	sessions, err := q.CountSessionProjectRows(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("count source sessions: %w", err)
	}
	prompts, err := q.CountPromptProjectRows(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("count source prompts: %w", err)
	}
	syncMutations, err := q.CountSyncMutationProjectRows(ctx, sqlNullString(from))
	if err != nil {
		return nil, fmt.Errorf("count source sync mutations: %w", err)
	}
	sourceProjectRows, err := q.CountProjectRows(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("count source project metadata: %w", err)
	}
	sourceEnrolled, err := q.IsProjectEnrolled(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("check source enrollment: %w", err)
	}
	destinationEnrolled, err := q.IsProjectEnrolled(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("check destination enrollment: %w", err)
	}

	return &ProjectMergePlan{
		From:                    source,
		To:                      destination,
		Observations:            observations,
		Sessions:                sessions,
		Prompts:                 prompts,
		SyncMutations:           syncMutations,
		SourceProjectRows:       sourceProjectRows,
		SourceEnrolled:          sourceEnrolled,
		DestinationEnrolled:     destinationEnrolled,
		WillCopyEnrollment:      sourceEnrolled && !destinationEnrolled,
		WillDeleteSourceProject: sourceProjectRows > 0,
	}, nil
}

func (s *Store) MergeProjects(from, to string) (*ProjectMergeResult, error) {
	plan, err := s.BuildProjectMergePlan(from, to)
	if err != nil {
		return nil, err
	}

	result := &ProjectMergeResult{
		Plan:                  *plan,
		Merged:                true,
		EnrollmentTransferred: plan.WillCopyEnrollment,
	}

	err = s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		ctx := context.Background()
		result.ObservationsUpdated = plan.Observations
		result.PromptsUpdated = plan.Prompts

		var err error
		result.SessionsUpdated, err = q.RenameSessionProject(ctx, dbgen.RenameSessionProjectParams{
			NewName: plan.To.ID,
			OldName: plan.From.ID,
		})
		if err != nil {
			return fmt.Errorf("merge sessions: %w", err)
		}

		result.SyncMutationsUpdated, err = q.RenameMutationProject(ctx, dbgen.RenameMutationProjectParams{
			NewName: sqlNullString(plan.To.ID),
			OldName: sqlNullString(plan.From.ID),
		})
		if err != nil {
			return fmt.Errorf("merge sync_mutations: %w", err)
		}

		if err = q.CopyProjectEnrollment(ctx, dbgen.CopyProjectEnrollmentParams{
			NewName: plan.To.ID,
			OldName: plan.From.ID,
		}); err != nil {
			return fmt.Errorf("merge sync_enrolled_projects insert: %w", err)
		}
		if err = q.DeleteProjectEnrollment(ctx, plan.From.ID); err != nil {
			return fmt.Errorf("merge sync_enrolled_projects delete: %w", err)
		}

		deleted, err := q.DeleteProjectByID(ctx, plan.From.ID)
		if err != nil {
			return fmt.Errorf("merge project metadata: %w", err)
		}
		result.SourceProjectDeleted = deleted > 0
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) BuildProjectRenamePlan(selector ProjectRenameSelector, newName string) (*ProjectRenamePlan, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}

	selector.ID = strings.TrimSpace(selector.ID)
	selector.Path = strings.TrimSpace(selector.Path)
	if (selector.ID == "") == (selector.Path == "") {
		return nil, fmt.Errorf("select exactly one project with id or path")
	}

	projects, err := s.ListProjectSummaries()
	if err != nil {
		return nil, err
	}

	var project ProjectSummary
	selectorName := "id"
	selectorValue := selector.ID
	if selector.ID != "" {
		found := false
		for _, candidate := range projects {
			if candidate.ID == selector.ID {
				project = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("project %q not found", selector.ID)
		}
	} else {
		selectorName = "path"
		selectorValue = normalizeProjectPath(selector.Path)
		var matches []ProjectSummary
		for _, candidate := range projects {
			if normalizeProjectPath(candidate.Directory) == selectorValue {
				matches = append(matches, candidate)
			}
		}
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("project path %q not found", selector.Path)
		case 1:
			project = matches[0]
		default:
			ids := make([]string, 0, len(matches))
			for _, match := range matches {
				ids = append(ids, match.ID)
			}
			return nil, fmt.Errorf("project path %q matches multiple projects: %s", selector.Path, strings.Join(ids, ", "))
		}
	}

	return &ProjectRenamePlan{
		Project:       project,
		Selector:      selectorName,
		SelectorValue: selectorValue,
		NewName:       newName,
		WillChange:    project.Name != newName,
	}, nil
}

func (s *Store) RenameProject(selector ProjectRenameSelector, newName string) (*ProjectRenameResult, error) {
	plan, err := s.BuildProjectRenamePlan(selector, newName)
	if err != nil {
		return nil, err
	}
	result := &ProjectRenameResult{Plan: *plan, Renamed: plan.WillChange}
	if !plan.WillChange {
		return result, nil
	}
	if err := s.q.UpsertProjectName(context.Background(), dbgen.UpsertProjectNameParams{
		ID:   plan.Project.ID,
		Name: plan.NewName,
	}); err != nil {
		return nil, fmt.Errorf("rename project metadata: %w", err)
	}
	return result, nil
}

func normalizeProjectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func (s *Store) MigrateProject(oldName, newName string) (*MigrateResult, error) {
	if oldName == "" || newName == "" || oldName == newName {
		return &MigrateResult{}, nil
	}

	exists, err := s.q.ProjectExists(context.Background(), oldName)
	if err != nil {
		return nil, fmt.Errorf("check old project: %w", err)
	}
	if !exists {
		return &MigrateResult{}, nil
	}

	result := &MigrateResult{Migrated: true}

	err = s.withTx(func(tx *sql.Tx) error {
		if err := s.ensureProjectTx(tx, newName); err != nil {
			return fmt.Errorf("ensure destination project: %w", err)
		}
		q := s.q.WithTx(tx)
		var err error
		result.ObservationsUpdated, err = q.CountObservationProjectRows(context.Background(), oldName)
		if err != nil {
			return fmt.Errorf("count observations: %w", err)
		}
		result.PromptsUpdated, err = q.CountPromptProjectRows(context.Background(), oldName)
		if err != nil {
			return fmt.Errorf("count prompts: %w", err)
		}

		result.SessionsUpdated, err = q.RenameSessionProject(context.Background(), dbgen.RenameSessionProjectParams{
			NewName: newName, OldName: oldName,
		})
		if err != nil {
			return fmt.Errorf("migrate sessions: %w", err)
		}

		result.SyncMutationsUpdated, err = q.RenameMutationProject(context.Background(), dbgen.RenameMutationProjectParams{
			NewName: sqlNullString(newName), OldName: sqlNullString(oldName),
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
		if _, err = q.DeleteProjectByID(context.Background(), oldName); err != nil {
			return fmt.Errorf("migrate project metadata: %w", err)
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
		if err := s.ensureProjectTx(tx, project); err != nil {
			return err
		}
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
