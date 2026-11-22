package idea

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository persists idea_history and chaos_history - the record of
// what's been suggested before (spec section 34's own "previous
// suggestions" input, section 36's "maintain a history so it doesn't
// repeatedly generate the same challenges"). Both tables already
// existed in the schema since 0001_init_schema.sql anticipated this
// phase; no migration was needed here.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// RecordSuggestions logs one "I HAVE NO IDEA" generation. suggestions'
// titles are stored as a JSON array (idea_history.suggestions_json)
// enough to reconstruct RecentTitles later, without needing to persist
// every field of every Suggestion (Reasons/Description are cheap to
// regenerate, the title is the only thing de-duplication needs).
func (r *Repository) RecordSuggestions(ctx context.Context, availableMinutes TimeMinutes, intent Intent, suggestions []Suggestion) (int64, error) {
	titles := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		titles = append(titles, s.Title)
	}
	b, err := json.Marshal(titles)
	if err != nil {
		return 0, fmt.Errorf("idea: encoding suggestion titles: %w", err)
	}

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO idea_history (generated_at, available_minutes, intent, suggestions_json)
		VALUES (?, ?, ?, ?)
	`, time.Now().Unix(), nullableMinutes(availableMinutes), nullableIntent(intent), string(b))
	if err != nil {
		return 0, fmt.Errorf("idea: recording suggestions: %w", err)
	}
	return res.LastInsertId()
}

// RecordAcceptance marks which of a past generation's suggestions (by
// its 0-based index - matching spec section 34's own numbered "1. / 2. /
// 3." display) the person actually went with. Purely a record for future
// "what's worked before" use; nothing currently reads it back besides
// history display, but persisting it now means a future phase can use it
// without a schema change.
func (r *Repository) RecordAcceptance(ctx context.Context, historyID int64, index int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE idea_history SET accepted_index = ? WHERE id = ?`, index, historyID)
	if err != nil {
		return fmt.Errorf("idea: recording acceptance for %d: %w", historyID, err)
	}
	return nil
}

// RecentTitles returns every suggestion title (across projects, goals,
// chaos, and generic fallbacks alike - pick.go's exclusion works purely
// on exact title match, source-agnostic) from the last `limit`
// generations, flattened into a set PickSuggestions/GenerateProjectIdea
// can check directly.
func (r *Repository) RecentTitles(ctx context.Context, limit int) (map[string]bool, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT suggestions_json FROM idea_history ORDER BY generated_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("idea: listing recent titles: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("idea: scanning suggestions_json: %w", err)
		}
		var titles []string
		if err := json.Unmarshal([]byte(raw), &titles); err != nil {
			continue // a malformed historical row must not break de-duplication for every future generation
		}
		for _, t := range titles {
			out[t] = true
		}
	}
	return out, rows.Err()
}

// RecordChaosShown logs that a chaos challenge was generated (section
// 36's own history requirement) - separate from acceptance, which is
// recorded by RecordChaosAcceptance once the person actually picks it.
func (r *Repository) RecordChaosShown(ctx context.Context, challenge string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO chaos_history (generated_at, challenge, accepted) VALUES (?, ?, 0)
	`, time.Now().Unix(), challenge)
	if err != nil {
		return 0, fmt.Errorf("idea: recording chaos challenge: %w", err)
	}
	return res.LastInsertId()
}

func (r *Repository) RecordChaosAcceptance(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE chaos_history SET accepted = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("idea: recording chaos acceptance for %d: %w", id, err)
	}
	return nil
}

// RecentChaosChallenges returns the exact challenge text of the last
// `limit` chaos generations, for the same title-exclusion mechanism
// RecentTitles provides for ordinary suggestions.
func (r *Repository) RecentChaosChallenges(ctx context.Context, limit int) (map[string]bool, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT challenge FROM chaos_history ORDER BY generated_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("idea: listing recent chaos challenges: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var challenge string
		if err := rows.Scan(&challenge); err != nil {
			return nil, fmt.Errorf("idea: scanning challenge: %w", err)
		}
		out[challenge] = true
	}
	return out, rows.Err()
}

func nullableMinutes(m TimeMinutes) any {
	if m == TimeWhatever {
		return nil
	}
	return int(m)
}

func nullableIntent(i Intent) any {
	if i == "" {
		return nil
	}
	return string(i)
}
