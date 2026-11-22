// Package goals implements section 18's goals - distinct from budgets:
// a goal targets an amount to REACH (e.g. "20 hours on Serenity this
// month"), where a budget limits an amount not to EXCEED.
package goals

import (
	"context"
	"database/sql"
	"fmt"
)

// Goal mirrors a goals row.
type Goal struct {
	ID          int64
	Name        string
	GoalType    string // "time" | "project" | "activity" | "custom"
	ProjectID   int64  // 0 = not project-scoped
	TargetValue float64
	Unit        string // "hours" (the only unit Phase 4's progress calculation understands)
	Period      string // "day" | "week" | "month" | "custom"
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListGoals(ctx context.Context) ([]Goal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, goal_type, COALESCE(project_id,0), target_value, unit, period
		FROM goals
		WHERE archived_at IS NULL
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("goals: listing: %w", err)
	}
	defer rows.Close()

	var out []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Name, &g.GoalType, &g.ProjectID, &g.TargetValue, &g.Unit, &g.Period); err != nil {
			return nil, fmt.Errorf("goals: scanning row: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// CreateGoal adds a new time-based or project-based goal. targetValue is
// in Unit (currently always "hours" - activity-based goals with a count
// unit are a documented future extension, not implemented: section 34's
// "Create 8 beats this month" style goal needs a way to count discrete
// events that doesn't exist yet).
func (r *Repository) CreateGoal(ctx context.Context, name, goalType string, projectID int64, targetValue float64, period string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("goals: name must not be empty")
	}
	switch goalType {
	case "time", "project", "activity", "custom":
	default:
		return 0, fmt.Errorf("goals: invalid goal type %q", goalType)
	}
	if targetValue <= 0 {
		return 0, fmt.Errorf("goals: target value must be positive")
	}
	var projectArg any
	if projectID != 0 {
		projectArg = projectID
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO goals (name, goal_type, project_id, target_value, unit, period, created_at)
		VALUES (?, ?, ?, ?, 'hours', ?, unixepoch())
	`, name, goalType, projectArg, targetValue, period)
	if err != nil {
		return 0, fmt.Errorf("goals: creating %q: %w", name, err)
	}
	return res.LastInsertId()
}

// ArchiveGoal soft-deletes a goal.
func (r *Repository) ArchiveGoal(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE goals SET archived_at = unixepoch() WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("goals: archiving %d: %w", id, err)
	}
	return nil
}
