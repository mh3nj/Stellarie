package services

import (
	"context"

	"stellarei/internal/categories"
)

type CategoryRecord struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ColorToken string `json:"colorToken"`
	IsBuiltin  bool   `json:"isBuiltin"`
}

type ApplicationCategoryRecord struct {
	ApplicationID     int64  `json:"applicationId"`
	ProcessName       string `json:"processName"`
	CategoryID        int64  `json:"categoryId"`
	DefaultCategoryID int64  `json:"defaultCategoryId"`
}

// CategoryRuleRecord is what ListRulesForCategory returns to the
// frontend - named distinctly from project_service.go's RuleRecord
// (same package, would otherwise collide; caught by the isolated
// app/services build check, not gofmt or a visual read - same failure
// mode as the toResultView collision noted in this project's earlier
// self-correction log).
type CategoryRuleRecord struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"categoryId"`
	Type       string `json:"type"`
	Pattern    string `json:"pattern"`
	Priority   int    `json:"priority"`
}

// CategoryService is bound to the frontend for the Analytics page's
// category management and application-mapping sections (spec section 10).
type CategoryService struct {
	repo *categories.Repository
}

func NewCategoryService(repo *categories.Repository) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) ListCategories() ([]CategoryRecord, error) {
	rows, err := s.repo.ListCategories(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]CategoryRecord, 0, len(rows))
	for _, c := range rows {
		out = append(out, CategoryRecord{ID: c.ID, Name: c.Name, ColorToken: c.ColorToken, IsBuiltin: c.IsBuiltin})
	}
	return out, nil
}

func (s *CategoryService) CreateCategory(name, colorToken string) (int64, error) {
	return s.repo.CreateCategory(context.Background(), name, colorToken)
}

func (s *CategoryService) DeleteCategory(id int64) error {
	return s.repo.DeleteCategory(context.Background(), id)
}

func (s *CategoryService) ListApplications() ([]ApplicationCategoryRecord, error) {
	rows, err := s.repo.ListApplications(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]ApplicationCategoryRecord, 0, len(rows))
	for _, a := range rows {
		out = append(out, ApplicationCategoryRecord{
			ApplicationID:     a.ApplicationID,
			ProcessName:       a.ProcessName,
			CategoryID:        a.CategoryID,
			DefaultCategoryID: a.DefaultCategoryID,
		})
	}
	return out, nil
}

func (s *CategoryService) SetApplicationCategory(applicationID, categoryID int64) error {
	return s.repo.SetApplicationCategory(context.Background(), applicationID, categoryID)
}

func (s *CategoryService) ListRulesForCategory(categoryID int64) ([]CategoryRuleRecord, error) {
	rows, err := s.repo.ListRulesForCategory(context.Background(), categoryID)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryRuleRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, CategoryRuleRecord{ID: r.ID, CategoryID: r.CategoryID, Type: string(r.Type), Pattern: r.Pattern, Priority: r.Priority})
	}
	return out, nil
}

// AddRule's ruleType must be one of "process", "window_title_contains",
// "path_contains", "domain" - see internal/categories.RuleType.
func (s *CategoryService) AddRule(categoryID int64, ruleType string, pattern string, priority int) (int64, error) {
	return s.repo.AddRule(context.Background(), categoryID, categories.RuleType(ruleType), pattern, priority)
}

func (s *CategoryService) DeleteRule(ruleID int64) error {
	return s.repo.DeleteRule(context.Background(), ruleID)
}
