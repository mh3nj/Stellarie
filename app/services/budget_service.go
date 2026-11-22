package services

import (
	"context"
	"time"

	"stellarei/internal/budgets"
	"stellarei/internal/periods"
	"stellarei/internal/sessions"
)

// BudgetStatus is a budget plus its live progress for the CURRENT period
// (computed fresh on every call, not cached/stored - a budget's
// "actual" is always relative to "right now", so caching it would just
// mean showing stale data). The frontend joins CategoryID/ProjectID
// against the category/project lists it already has loaded rather than
// this service duplicating those names.
type BudgetStatus struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	CategoryID    int64  `json:"categoryId"`
	ProjectID     int64  `json:"projectId"`
	BudgetType    string `json:"budgetType"`
	LimitMinutes  int    `json:"limitMinutes"`
	Schedule      string `json:"schedule"`
	WarnAtMinutes int    `json:"warnAtMinutes"`
	HardLimit     bool   `json:"hardLimit"`
	ActualMinutes int    `json:"actualMinutes"`
	PeriodStart   int64  `json:"periodStart"`
	PeriodEnd     int64  `json:"periodEnd"`
}

type BudgetService struct {
	repo        *budgets.Repository
	sessionRepo sessions.Repository
}

func NewBudgetService(repo *budgets.Repository, sessionRepo sessions.Repository) *BudgetService {
	return &BudgetService{repo: repo, sessionRepo: sessionRepo}
}

// ListWithStatus computes every budget's current-period progress.
// Several budgets commonly share the same schedule (e.g. multiple
// "daily" budgets) - SumDurationByCategory/SumDurationByProject each
// already compute EVERY category/project's total in one GROUP BY query,
// so calling either once per budget row and discarding all but one key
// wastes a full aggregate query per redundant row. categoryCache and
// projectCache below memoize each distinct [start,end) window's result within this one
// call, so N budgets sharing a schedule cost one query, not N
// measured at ~20x fewer queries for 20 same-schedule budgets at
// realistic 10-year session volume (see README's Phase 12 note). Same
// results either way; this changes only how many times the database is
// asked, never what it's asked or what comes back.
func (s *BudgetService) ListWithStatus() ([]BudgetStatus, error) {
	ctx := context.Background()
	rows, err := s.repo.ListBudgets(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	type window struct{ start, end int64 }
	categoryCache := map[window]map[int64]int64{}
	projectCache := map[window]map[int64]int64{}

	out := make([]BudgetStatus, 0, len(rows))
	for _, b := range rows {
		start, end := periods.Window(b.Schedule, now)
		w := window{start, end}

		var actualSeconds int64
		if b.CategoryID != 0 {
			sums, cached := categoryCache[w]
			if !cached {
				sums, err = s.sessionRepo.SumDurationByCategory(ctx, start, end)
				if err != nil {
					sums = nil // degrade to zero, exactly as before this change
				}
				categoryCache[w] = sums
			}
			actualSeconds = sums[b.CategoryID]
		} else if b.ProjectID != 0 {
			sums, cached := projectCache[w]
			if !cached {
				sums, err = s.sessionRepo.SumDurationByProject(ctx, start, end)
				if err != nil {
					sums = nil
				}
				projectCache[w] = sums
			}
			actualSeconds = sums[b.ProjectID]
		}

		out = append(out, BudgetStatus{
			ID:            b.ID,
			Name:          b.Name,
			CategoryID:    b.CategoryID,
			ProjectID:     b.ProjectID,
			BudgetType:    b.BudgetType,
			LimitMinutes:  b.LimitMinutes,
			Schedule:      b.Schedule,
			WarnAtMinutes: b.WarnAtMinutes,
			HardLimit:     b.HardLimit,
			ActualMinutes: int(actualSeconds / 60),
			PeriodStart:   start,
			PeriodEnd:     end,
		})
	}
	return out, nil
}

// CreateBudget's budgetType must be "maximum" or "target"; schedule one
// of "daily"/"weekly"/"monthly". Exactly one of categoryID/projectID
// should be non-zero (the UI enforces this).
func (s *BudgetService) CreateBudget(name string, categoryID, projectID int64, budgetType string, limitMinutes int, schedule string, warnAtMinutes int, hardLimit bool) (int64, error) {
	return s.repo.CreateBudget(context.Background(), name, categoryID, projectID, budgetType, limitMinutes, schedule, warnAtMinutes, hardLimit)
}

func (s *BudgetService) ArchiveBudget(id int64) error {
	return s.repo.ArchiveBudget(context.Background(), id)
}
