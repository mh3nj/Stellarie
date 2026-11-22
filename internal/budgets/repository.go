// Package budgets implements time budgets (spec section 17): a
// category- or project-scoped limit or target evaluated against the
// current day/week/month, per internal/periods.Window.
package budgets

import (
	"context"
	"database/sql"
	"fmt"
)

// Budget mirrors a budgets row.
type Budget struct {
	ID            int64
	Name          string
	CategoryID    int64  // 0 = not category-scoped
	ProjectID     int64  // 0 = not project-scoped
	BudgetType    string // "maximum" | "target"
	LimitMinutes  int
	Schedule      string // "daily" | "weekly" | "monthly" | "custom"
	WarnAtMinutes int    // 0 = no warning threshold configured
	HardLimit     bool
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListBudgets(ctx context.Context) ([]Budget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(category_id,0), COALESCE(project_id,0), budget_type,
		       limit_minutes, schedule, COALESCE(warn_at_minutes,0), hard_limit
		FROM budgets
		WHERE archived_at IS NULL
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("budgets: listing: %w", err)
	}
	defer rows.Close()

	var out []Budget
	for rows.Next() {
		var b Budget
		var hardLimit int
		if err := rows.Scan(&b.ID, &b.Name, &b.CategoryID, &b.ProjectID, &b.BudgetType,
			&b.LimitMinutes, &b.Schedule, &b.WarnAtMinutes, &hardLimit); err != nil {
			return nil, fmt.Errorf("budgets: scanning row: %w", err)
		}
		b.HardLimit = hardLimit != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

// CreateBudget adds a new budget. Exactly one of categoryID/projectID
// should be non-zero in practice (the UI enforces this), though the
// schema itself doesn't forbid both or neither - an unscoped budget
// simply never matches any session when its progress is computed.
func (r *Repository) CreateBudget(ctx context.Context, name string, categoryID, projectID int64, budgetType string, limitMinutes int, schedule string, warnAtMinutes int, hardLimit bool) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("budgets: name must not be empty")
	}
	switch budgetType {
	case "maximum", "target":
	default:
		return 0, fmt.Errorf("budgets: invalid budget type %q", budgetType)
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO budgets (name, category_id, project_id, budget_type, limit_minutes, schedule, warn_at_minutes, hard_limit, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
	`, name, nullableID(categoryID), nullableID(projectID), budgetType, limitMinutes, schedule, nullableMinutes(warnAtMinutes), boolToInt(hardLimit))
	if err != nil {
		return 0, fmt.Errorf("budgets: creating %q: %w", name, err)
	}
	return res.LastInsertId()
}

// ArchiveBudget soft-deletes a budget.
func (r *Repository) ArchiveBudget(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE budgets SET archived_at = unixepoch() WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("budgets: archiving %d: %w", id, err)
	}
	return nil
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullableMinutes(m int) any {
	if m == 0 {
		return nil
	}
	return m
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
