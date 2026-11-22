package services

import (
	"context"

	"stellarei/internal/projects"
)

// ProjectRecord is what ListProjects returns to the frontend - plain
// JSON-friendly fields rather than exposing internal/projects.Project's
// *int64 ArchivedAt directly (Wails binding codegen handles pointers
// fine, but a bool is simpler for the frontend to branch on).
type ProjectRecord struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ColorToken  string `json:"colorToken"`
	Archived    bool   `json:"archived"`
}

// RuleRecord is what ListRules returns to the frontend.
type RuleRecord struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	Type      string `json:"type"`
	Pattern   string `json:"pattern"`
	Priority  int    `json:"priority"`
}

// ProjectService is bound to the frontend for the Projects page (and the
// Timeline's project picker): CRUD projects, CRUD their association
// rules. The Activity Engine doesn't go through this service - it uses
// internal/projects.Resolver directly (see main.go) - this exists purely
// for the UI side of section 11/65.
type ProjectService struct {
	repo *projects.Repository
}

func NewProjectService(repo *projects.Repository) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) ListProjects(includeArchived bool) ([]ProjectRecord, error) {
	rows, err := s.repo.ListProjects(context.Background(), includeArchived)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectRecord, 0, len(rows))
	for _, p := range rows {
		out = append(out, ProjectRecord{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Status:      p.Status,
			ColorToken:  p.ColorToken,
			Archived:    p.ArchivedAt != nil,
		})
	}
	return out, nil
}

func (s *ProjectService) CreateProject(name, description, colorToken string) (int64, error) {
	return s.repo.CreateProject(context.Background(), name, description, colorToken)
}

func (s *ProjectService) UpdateProject(id int64, name, description, colorToken, status string) error {
	return s.repo.UpdateProject(context.Background(), id, name, description, colorToken, status)
}

func (s *ProjectService) ArchiveProject(id int64) error {
	return s.repo.ArchiveProject(context.Background(), id)
}

func (s *ProjectService) UnarchiveProject(id int64) error {
	return s.repo.UnarchiveProject(context.Background(), id)
}

func (s *ProjectService) ListRulesForProject(projectID int64) ([]RuleRecord, error) {
	rows, err := s.repo.ListRulesForProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	out := make([]RuleRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, RuleRecord{ID: r.ID, ProjectID: r.ProjectID, Type: string(r.Type), Pattern: r.Pattern, Priority: r.Priority})
	}
	return out, nil
}

// AddRule's ruleType must be one of "process", "window_title_contains",
// "path_contains", "domain" - see internal/projects.RuleType.
func (s *ProjectService) AddRule(projectID int64, ruleType string, pattern string, priority int) (int64, error) {
	return s.repo.AddRule(context.Background(), projectID, projects.RuleType(ruleType), pattern, priority)
}

func (s *ProjectService) DeleteRule(ruleID int64) error {
	return s.repo.DeleteRule(context.Background(), ruleID)
}
