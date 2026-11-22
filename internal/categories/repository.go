// Package categories implements the Application Registry's category
// assignment (spec section 10) and the Category resolver the Activity
// Engine uses when closing a session. As of category_rules
// (migrations/0007), categories now have the same pattern-matching rule
// engine projects always had (spec section 65's own example - "IF
// domain = youtube.com THEN category = Entertainment" - is a category
// rule) via rules.go/Resolver, checked before falling back to the
// simpler per-application default_category_id/override model this
// package started with.
package categories

import (
	"context"
	"database/sql"
	"fmt"
)

// Category mirrors a categories row.
type Category struct {
	ID         int64
	Name       string
	ColorToken string
	IsBuiltin  bool
}

// ApplicationCategory is one row of the Application Registry's
// category-relevant fields - what the category-assignment UI (section
// 10) lists and edits.
type ApplicationCategory struct {
	ApplicationID     int64
	ProcessName       string
	CategoryID        int64 // 0 = falls back to DefaultCategoryID
	DefaultCategoryID int64 // 0 = uncategorized
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, COALESCE(color_token,''), is_builtin FROM categories ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("categories: listing: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		var isBuiltin int
		if err := rows.Scan(&c.ID, &c.Name, &c.ColorToken, &isBuiltin); err != nil {
			return nil, fmt.Errorf("categories: scanning row: %w", err)
		}
		c.IsBuiltin = isBuiltin != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory adds a user-defined (non-builtin) category.
func (r *Repository) CreateCategory(ctx context.Context, name, colorToken string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("categories: name must not be empty")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO categories (name, color_token, is_builtin, created_at)
		VALUES (?, ?, 0, unixepoch())
	`, name, colorToken)
	if err != nil {
		return 0, fmt.Errorf("categories: creating %q: %w", name, err)
	}
	return res.LastInsertId()
}

// DeleteCategory removes a user-defined category. Built-in categories
// can't be deleted - every application/session/event referencing it
// falls back to NULL (ON DELETE SET NULL) rather than losing the row
// silently.
func (r *Repository) DeleteCategory(ctx context.Context, id int64) error {
	var isBuiltin int
	if err := r.db.QueryRowContext(ctx, `SELECT is_builtin FROM categories WHERE id = ?`, id).Scan(&isBuiltin); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("categories: no category with id %d", id)
		}
		return fmt.Errorf("categories: checking %d: %w", id, err)
	}
	if isBuiltin != 0 {
		return fmt.Errorf("categories: built-in categories cannot be deleted")
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id); err != nil {
		return fmt.Errorf("categories: deleting %d: %w", id, err)
	}
	return nil
}

// ListApplications returns every known application (the Activity Engine
// auto-registers these as it sees new processes - internal/activity's
// Repository.ResolveApplicationID) with its current category assignment,
// for the category-mapping UI.
func (r *Repository) ListApplications(ctx context.Context) ([]ApplicationCategory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, process_name, COALESCE(category_id, 0), COALESCE(default_category_id, 0)
		FROM applications
		ORDER BY process_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("categories: listing applications: %w", err)
	}
	defer rows.Close()

	var out []ApplicationCategory
	for rows.Next() {
		var a ApplicationCategory
		if err := rows.Scan(&a.ApplicationID, &a.ProcessName, &a.CategoryID, &a.DefaultCategoryID); err != nil {
			return nil, fmt.Errorf("categories: scanning application row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetApplicationCategory sets (or clears, with categoryID 0) an
// application's category override (section 10: "The user must be able
// to override automatic classifications").
func (r *Repository) SetApplicationCategory(ctx context.Context, applicationID, categoryID int64) error {
	var arg any
	if categoryID != 0 {
		arg = categoryID
	}
	_, err := r.db.ExecContext(ctx, `UPDATE applications SET category_id = ? WHERE id = ?`, arg, applicationID)
	if err != nil {
		return fmt.Errorf("categories: setting category for application %d: %w", applicationID, err)
	}
	return nil
}

// CategoryForApplication resolves the effective category for an
// application: its explicit override if set, else its default, else 0
// (uncategorized - the session simply gets no category, same as an
// unassigned project).
func (r *Repository) CategoryForApplication(ctx context.Context, applicationID int64) (int64, error) {
	var categoryID, defaultCategoryID sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT category_id, default_category_id FROM applications WHERE id = ?`, applicationID,
	).Scan(&categoryID, &defaultCategoryID)
	if err == sql.ErrNoRows {
		return 0, nil // an application id that doesn't exist yet - no category, not an error
	}
	if err != nil {
		return 0, fmt.Errorf("categories: resolving for application %d: %w", applicationID, err)
	}
	if categoryID.Valid {
		return categoryID.Int64, nil
	}
	if defaultCategoryID.Valid {
		return defaultCategoryID.Int64, nil
	}
	return 0, nil
}

// RuleRecord is a category_rules row, including its own id (needed for
// deletion) alongside the pure Rule shape Evaluate consumes - same
// shape as internal/projects.RuleRecord.
type RuleRecord struct {
	ID int64
	Rule
}

// ListRulesForCategory returns a category's rules ordered by priority
// ascending - already in the order Evaluate expects.
func (r *Repository) ListRulesForCategory(ctx context.Context, categoryID int64) ([]RuleRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, category_id, rule_type, pattern, priority
		FROM category_rules
		WHERE category_id = ? AND enabled = 1
		ORDER BY priority ASC, id ASC
	`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("categories: listing rules for %d: %w", categoryID, err)
	}
	defer rows.Close()

	var out []RuleRecord
	for rows.Next() {
		var rec RuleRecord
		var ruleType string
		if err := rows.Scan(&rec.ID, &rec.CategoryID, &ruleType, &rec.Pattern, &rec.Priority); err != nil {
			return nil, fmt.Errorf("categories: scanning rule row: %w", err)
		}
		rec.Type = RuleType(ruleType)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ListAllRules returns every enabled rule across every category, ordered
// by priority ascending - what Resolver loads to evaluate against a
// closing session.
func (r *Repository) ListAllRules(ctx context.Context) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT cr.category_id, cr.rule_type, cr.pattern, cr.priority
		FROM category_rules cr
		JOIN categories c ON c.id = cr.category_id
		WHERE cr.enabled = 1
		ORDER BY cr.priority ASC, cr.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("categories: listing all rules: %w", err)
	}
	defer rows.Close()

	var out []Rule
	for rows.Next() {
		var rule Rule
		var ruleType string
		if err := rows.Scan(&rule.CategoryID, &ruleType, &rule.Pattern, &rule.Priority); err != nil {
			return nil, fmt.Errorf("categories: scanning rule row: %w", err)
		}
		rule.Type = RuleType(ruleType)
		out = append(out, rule)
	}
	return out, rows.Err()
}

// AddRule creates a new rule for a category. ruleType must be one of the
// RuleType constants.
func (r *Repository) AddRule(ctx context.Context, categoryID int64, ruleType RuleType, pattern string, priority int) (int64, error) {
	switch ruleType {
	case RuleProcess, RuleWindowTitleContains, RulePathContains, RuleDomain:
	default:
		return 0, fmt.Errorf("categories: invalid rule type %q", ruleType)
	}
	if pattern == "" {
		return 0, fmt.Errorf("categories: rule pattern must not be empty")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO category_rules (category_id, rule_type, pattern, priority, enabled, created_at)
		VALUES (?, ?, ?, ?, 1, unixepoch())
	`, categoryID, string(ruleType), pattern, priority)
	if err != nil {
		return 0, fmt.Errorf("categories: adding rule: %w", err)
	}
	return res.LastInsertId()
}

// DeleteRule removes a rule by id.
func (r *Repository) DeleteRule(ctx context.Context, ruleID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM category_rules WHERE id = ?`, ruleID); err != nil {
		return fmt.Errorf("categories: deleting rule %d: %w", ruleID, err)
	}
	return nil
}
