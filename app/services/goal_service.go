package services

import (
	"context"
	"time"

	"stellarei/internal/goals"
	"stellarei/internal/periods"
	"stellarei/internal/sessions"
)

type GoalStatus struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	GoalType        string  `json:"goalType"`
	ProjectID       int64   `json:"projectId"`
	TargetValue     float64 `json:"targetValue"`
	Unit            string  `json:"unit"`
	Period          string  `json:"period"`
	ActualHours     float64 `json:"actualHours"`
	PercentComplete float64 `json:"percentComplete"`
	PeriodStart     int64   `json:"periodStart"`
	PeriodEnd       int64   `json:"periodEnd"`
}

type GoalService struct {
	repo        *goals.Repository
	sessionRepo sessions.Repository
}

func NewGoalService(repo *goals.Repository, sessionRepo sessions.Repository) *GoalService {
	return &GoalService{repo: repo, sessionRepo: sessionRepo}
}

// ListWithStatus computes every goal's current-period progress. See
// BudgetService.ListWithStatus's identical doc comment: several goals
// commonly share a period (e.g. multiple "monthly" goals), and
// SumDurationByProject already computes every project's total in one
// query - projectCache below memoizes it per distinct [start,end)
// window so N same-period goals cost one query, not N.
func (s *GoalService) ListWithStatus() ([]GoalStatus, error) {
	ctx := context.Background()
	rows, err := s.repo.ListGoals(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	type window struct{ start, end int64 }
	projectCache := map[window]map[int64]int64{}

	out := make([]GoalStatus, 0, len(rows))
	for _, g := range rows {
		start, end := periods.Window(g.Period, now)
		w := window{start, end}

		var actualSeconds int64
		if g.ProjectID != 0 {
			sums, cached := projectCache[w]
			if !cached {
				sums, err = s.sessionRepo.SumDurationByProject(ctx, start, end)
				if err != nil {
					sums = nil // degrade to zero, exactly as before this change
				}
				projectCache[w] = sums
			}
			actualSeconds = sums[g.ProjectID]
		}
		// goal_type "time"/"activity"/"custom" without a project scope
		// has no session data to sum against yet - section 34's
		// activity-count goals ("Create 8 beats") need a way to count
		// discrete events that doesn't exist yet (see
		// internal/goals.Repository.CreateGoal's doc comment). Such
		// goals list with ActualHours 0 rather than being hidden.

		actualHours := float64(actualSeconds) / 3600
		percent := 0.0
		if g.TargetValue > 0 {
			percent = (actualHours / g.TargetValue) * 100
			if percent > 100 {
				percent = 100
			}
		}

		out = append(out, GoalStatus{
			ID:              g.ID,
			Name:            g.Name,
			GoalType:        g.GoalType,
			ProjectID:       g.ProjectID,
			TargetValue:     g.TargetValue,
			Unit:            g.Unit,
			Period:          g.Period,
			ActualHours:     actualHours,
			PercentComplete: percent,
			PeriodStart:     start,
			PeriodEnd:       end,
		})
	}
	return out, nil
}

// CreateGoal's goalType must be "time", "project", "activity", or
// "custom"; period one of "day"/"week"/"month". targetValue is in hours.
func (s *GoalService) CreateGoal(name, goalType string, projectID int64, targetHours float64, period string) (int64, error) {
	return s.repo.CreateGoal(context.Background(), name, goalType, projectID, targetHours, period)
}

func (s *GoalService) ArchiveGoal(id int64) error {
	return s.repo.ArchiveGoal(context.Background(), id)
}
