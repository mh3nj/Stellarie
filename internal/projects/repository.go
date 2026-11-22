package projects

import (
	"context"
	"database/sql"
	"fmt"
)

// Project mirrors a row of the projects table (spec section 11). Fields
// map directly to migrations/0001_init_schema.sql's projects table.
type Project struct {
	ID          int64
	Name        string
	Description string
	Status      string // active | paused | completed | abandoned
	ColorToken  string
	CreatedAt   int64
	ArchivedAt  *int64
}

// Repository is the only path to the projects/project_rules tables.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ListProjects returns every non-archived project by default, or every
// project (including archived) when includeArchived is true - the
// Projects page needs the latter for an "Archived" section, but the
// Timeline's project picker and the rule editor should never offer an
// archived project by accident.
func (r *Repository) ListProjects(ctx context.Context, includeArchived bool) ([]Project, error) {
	query := `SELECT id, name, description, status, color_token, created_at, archived_at FROM projects`
	if !includeArchived {
		query += ` WHERE archived_at IS NULL`
	}
	query += ` ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("projects: listing: %w", err)
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		var description, colorToken sql.NullString
		var archivedAt sql.NullInt64
		if err := rows.Scan(&p.ID, &p.Name, &description, &p.Status, &colorToken, &p.CreatedAt, &archivedAt); err != nil {
			return nil, fmt.Errorf("projects: scanning row: %w", err)
		}
		p.Description = description.String
		p.ColorToken = colorToken.String
		if archivedAt.Valid {
			v := archivedAt.Int64
			p.ArchivedAt = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProject inserts a new active project and returns its id.
func (r *Repository) CreateProject(ctx context.Context, name, description, colorToken string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("projects: name must not be empty")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (name, description, color_token, status, created_at)
		VALUES (?, ?, ?, 'active', unixepoch())
	`, name, description, colorToken)
	if err != nil {
		return 0, fmt.Errorf("projects: creating %q: %w", name, err)
	}
	return res.LastInsertId()
}

// UpdateProject changes a project's editable fields. status must be one
// of active/paused/completed/abandoned.
func (r *Repository) UpdateProject(ctx context.Context, id int64, name, description, colorToken, status string) error {
	switch status {
	case "active", "paused", "completed", "abandoned":
	default:
		return fmt.Errorf("projects: invalid status %q", status)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE projects SET name = ?, description = ?, color_token = ?, status = ?
		WHERE id = ?
	`, name, description, colorToken, status, id)
	if err != nil {
		return fmt.Errorf("projects: updating %d: %w", id, err)
	}
	return nil
}

// ArchiveProject soft-deletes a project (archived_at set, row kept
// existing sessions/rules referencing it stay intact per the schema's
// ON DELETE SET NULL, but archiving doesn't even trigger that; the
// project row itself is never removed here).
func (r *Repository) ArchiveProject(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE projects SET archived_at = unixepoch() WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("projects: archiving %d: %w", id, err)
	}
	return nil
}

// UnarchiveProject reverses ArchiveProject.
func (r *Repository) UnarchiveProject(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE projects SET archived_at = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("projects: unarchiving %d: %w", id, err)
	}
	return nil
}

// RuleRecord is a project_rules row, including its own id (needed for
// deletion) alongside the pure Rule shape Evaluate consumes.
type RuleRecord struct {
	ID int64
	Rule
}

// ListRulesForProject returns a project's rules ordered by priority
// ascending - already in the order Evaluate expects.
func (r *Repository) ListRulesForProject(ctx context.Context, projectID int64) ([]RuleRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, rule_type, pattern, priority
		FROM project_rules
		WHERE project_id = ? AND enabled = 1
		ORDER BY priority ASC, id ASC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("projects: listing rules for %d: %w", projectID, err)
	}
	defer rows.Close()

	var out []RuleRecord
	for rows.Next() {
		var rec RuleRecord
		var ruleType string
		if err := rows.Scan(&rec.ID, &rec.ProjectID, &ruleType, &rec.Pattern, &rec.Priority); err != nil {
			return nil, fmt.Errorf("projects: scanning rule row: %w", err)
		}
		rec.Type = RuleType(ruleType)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ListAllRules returns every enabled rule across every project, ordered
// by priority ascending - what Resolver loads to evaluate against a
// session (a session could belong to any project, so it needs the full
// set, not one project's).
func (r *Repository) ListAllRules(ctx context.Context) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.project_id, pr.rule_type, pr.pattern, pr.priority
		FROM project_rules pr
		JOIN projects p ON p.id = pr.project_id
		WHERE pr.enabled = 1 AND p.archived_at IS NULL
		ORDER BY pr.priority ASC, pr.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("projects: listing all rules: %w", err)
	}
	defer rows.Close()

	var out []Rule
	for rows.Next() {
		var rule Rule
		var ruleType string
		if err := rows.Scan(&rule.ProjectID, &ruleType, &rule.Pattern, &rule.Priority); err != nil {
			return nil, fmt.Errorf("projects: scanning rule row: %w", err)
		}
		rule.Type = RuleType(ruleType)
		out = append(out, rule)
	}
	return out, rows.Err()
}

// AddRule creates a new rule for a project. ruleType must be one of the
// RuleType constants.
func (r *Repository) AddRule(ctx context.Context, projectID int64, ruleType RuleType, pattern string, priority int) (int64, error) {
	switch ruleType {
	case RuleProcess, RuleWindowTitleContains, RulePathContains, RuleDomain:
	default:
		return 0, fmt.Errorf("projects: invalid rule type %q", ruleType)
	}
	if pattern == "" {
		return 0, fmt.Errorf("projects: rule pattern must not be empty")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO project_rules (project_id, rule_type, pattern, priority, enabled, created_at)
		VALUES (?, ?, ?, ?, 1, unixepoch())
	`, projectID, string(ruleType), pattern, priority)
	if err != nil {
		return 0, fmt.Errorf("projects: adding rule: %w", err)
	}
	return res.LastInsertId()
}

// DeleteRule removes a rule by id.
func (r *Repository) DeleteRule(ctx context.Context, ruleID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM project_rules WHERE id = ?`, ruleID); err != nil {
		return fmt.Errorf("projects: deleting rule %d: %w", ruleID, err)
	}
	return nil
}
