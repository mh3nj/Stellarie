package services

// Regression test for the 2026-09-15 black-screen crash: on a fresh
// database (no goals, no budgets, no sessions), several services
// returned Go nil slices, which encoding/json marshals as `null` - and
// the frontend reads .length/.map off those fields directly, so one
// null array unmounted the entire app (reached via Calendar day clicks
// into Time Receipt, whose hash route then re-crashed every reload).
//
// Contract enforced here: every slice a bound service method returns
// top-level or nested in a view - must marshal as a JSON array, never
// null, even when there is nothing in it. Each case runs against a
// FRESH temp database (fully migrated, completely empty) to hit the
// exact shape that crashed.
//
// NOTE: this is deliberately the first test file in app/services,
// which otherwise has no suite by convention (testable logic lives in
// internal/). A convention is not worth a second black screen - this
// contract sits at the service/JSON boundary, which exists nowhere
// else, so it is tested here or nowhere.
import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"stellarei/internal/budgets"
	"stellarei/internal/categories"
	"stellarei/internal/database"
	"stellarei/internal/goals"
	"stellarei/internal/projects"
	"stellarei/internal/sessions"
)

func openEmptyTestDB(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "empty.db")})
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return ctx, db
}

// assertJSONArray marshals view and requires every named key to be a
// JSON array (possibly empty), failing on null/missing/other.
func assertJSONArray(t *testing.T, view any, keys ...string) {
	t.Helper()
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshaling view: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		t.Fatalf("re-reading marshaled view: %v", err)
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			t.Errorf("key %q missing from marshaled view", k)
			continue
		}
		arr, ok := v.([]any)
		if !ok || arr == nil {
			t.Errorf("key %q must marshal as a JSON array, got %v", k, v)
		}
		_ = arr
	}
}

func TestEmptyDBViewsMarshalArraysNotNull(t *testing.T) {
	ctx, db := openEmptyTestDB(t)

	sessionRepo := sessions.NewRepository(db)
	projectRepo := projects.NewRepository(db)
	categoryRepo := categories.NewRepository(db)
	goalService := NewGoalService(goals.NewRepository(db), sessionRepo)
	budgetService := NewBudgetService(budgets.NewRepository(db), sessionRepo)

	t.Run("receipt", func(t *testing.T) {
		svc := NewTimeReceiptService(sessionRepo, goalService, budgetService, nil)
		view, err := svc.GetTimeReceipt(time.Now().Unix())
		if err != nil {
			t.Fatalf("GetTimeReceipt: %v", err)
		}
		assertJSONArray(t, view, "categories", "goals", "budgets")
	})

	t.Run("myyear", func(t *testing.T) {
		svc := NewMyYearService(sessionRepo, projectRepo, goalService, nil)
		view, err := svc.GetYear(time.Now().Year())
		if err != nil {
			t.Fatalf("GetYear: %v", err)
		}
		assertJSONArray(t, view, "categories", "topProjects", "longestSessions",
			"abandonedProjects", "goalsCurrentlyComplete")
	})

	t.Run("search with no data", func(t *testing.T) {
		svc := NewSearchService(sessionRepo, projectRepo, categoryRepo)
		// Free-text path with zero sessions.
		view, err := svc.Search("longest session")
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		assertJSONArray(t, view, "results")
		// The "nothing found" interpretation must still be present
		// an empty array is not an error.
		if !strings.Contains(strings.ToLower(view.Interpretation), "nothing found") {
			t.Errorf("expected a nothing-found interpretation, got %q", view.Interpretation)
		}
		// Superlative path with a RECOGNIZED entity (Development is
		// seeded by migration 0004) but zero sessions - this is the
		// rankAndLimit empty-superlative branch that used to return
		// an explicit nil.
		view, err = svc.Search("longest Development session")
		if err != nil {
			t.Fatalf("Search superlative: %v", err)
		}
		assertJSONArray(t, view, "results")
	})

	t.Run("themes emptied", func(t *testing.T) {
		settings := NewSettingsService(db)
		svc := NewThemeService(db, settings)
		if _, err := db.ExecContext(ctx, `DELETE FROM themes`); err != nil {
			t.Fatalf("emptying themes: %v", err)
		}
		themes, err := svc.ListThemes()
		if err != nil {
			t.Fatalf("ListThemes: %v", err)
		}
		payload, err := json.Marshal(themes)
		if err != nil {
			t.Fatalf("marshaling themes: %v", err)
		}
		if string(payload) != "[]" {
			t.Errorf("empty theme list must marshal as [], got %s", payload)
		}
	})
}
