package patterns

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository persists Insights into the pattern_insights table 0001
// already created - this package's first writer. insight_type is the
// natural key the caller derives (app/services/pattern_service.go, as
// "<kind>:<categoryId>", e.g. "typical_start:3"), made UNIQUE by
// migration 0006 specifically so Sync below can upsert instead of
// accumulating a fresh row every time Pattern Discovery recomputes
// and so a dismissed insight_type can never be silently resurrected by
// that same recompute. See 0006_pattern_insights_dismissal.sql's own
// comment for the full rationale.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Sync upserts insightType's latest body/timestamp - creating the row
// on first sight, refreshing it on every later sight - UNLESS the
// person has already dismissed that insight_type, in which case this is
// a deliberate no-op: the row is left exactly as it was when dismissed,
// never resurrected with fresh wording. Verified against real SQLite
// (the conditional `ON CONFLICT ... DO UPDATE ... WHERE dismissed = 0`
// clause) before this Go code was written - see this repository's test
// file.
func (r *Repository) Sync(ctx context.Context, insightType, body string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pattern_insights (insight_type, body, generated_at, dismissed)
		VALUES (?, ?, ?, 0)
		ON CONFLICT(insight_type) DO UPDATE SET
			body = excluded.body,
			generated_at = excluded.generated_at
		WHERE pattern_insights.dismissed = 0
	`, insightType, body, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("patterns: syncing %q: %w", insightType, err)
	}
	return nil
}

// Dismiss hides insightType going forward - GetInsights (the service
// layer) filters it out of what it shows, and Sync stops updating its
// stored body until Undismiss is called.
func (r *Repository) Dismiss(ctx context.Context, insightType string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE pattern_insights SET dismissed = 1 WHERE insight_type = ?`, insightType)
	if err != nil {
		return fmt.Errorf("patterns: dismissing %q: %w", insightType, err)
	}
	return nil
}

// Undismiss reverses Dismiss, so the next Sync (and thus the next
// GetInsights call, if the underlying pattern is still true) shows it
// again with current data - not stale, dismissed-era wording.
func (r *Repository) Undismiss(ctx context.Context, insightType string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE pattern_insights SET dismissed = 0 WHERE insight_type = ?`, insightType)
	if err != nil {
		return fmt.Errorf("patterns: undismissing %q: %w", insightType, err)
	}
	return nil
}

// DismissedTypes returns every insight_type currently dismissed, as a
// set the service layer checks each freshly-computed Insight against
// before including it in what's shown - a dismissed pattern stays
// hidden even after the data behind it changes and would otherwise
// regenerate an insight with the same type but different wording.
func (r *Repository) DismissedTypes(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT insight_type FROM pattern_insights WHERE dismissed = 1`)
	if err != nil {
		return nil, fmt.Errorf("patterns: listing dismissed types: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("patterns: scanning dismissed insight_type: %w", err)
		}
		out[t] = true
	}
	return out, rows.Err()
}
