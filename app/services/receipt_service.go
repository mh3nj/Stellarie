package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stellarei/internal/ai"
	"stellarei/internal/periods"
	"stellarei/internal/sessions"
)

// TimeReceiptGoalLine is one row of the receipt's GOALS section (spec
// section 60's own example: "Development ✓ / Music ✗"). Met reflects
// each goal's CURRENT status against its own configured period (daily,
// weekly, monthly - see GoalService.ListWithStatus), not specifically
// "was this goal completed today" - a monthly goal doesn't reset a
// checkmark every day. The frontend should caption this honestly (e.g.
// "current progress") rather than implying every line is a same-day
// fact, matching section 66's "never invent statistics" / "label
// estimates" rule applied to framing, not just numbers.
type TimeReceiptGoalLine struct {
	Name            string  `json:"name"`
	Met             bool    `json:"met"`
	PercentComplete float64 `json:"percentComplete"`
}

// TimeReceiptBudgetLine is one row of the receipt's BUDGETS section
// (spec section 60's "Gaming +54m" example). Deliberately restricted to
// budgets whose Schedule is "daily" - a weekly or monthly budget's
// ActualMinutes accumulates across days, so labeling it on a single
// day's receipt would misstate what that number means. OverMinutes is
// positive when over the limit, negative when under it (spec's own
// "+54m" sign convention).
type TimeReceiptBudgetLine struct {
	Name          string `json:"name"`
	LimitMinutes  int    `json:"limitMinutes"`
	ActualMinutes int    `json:"actualMinutes"`
	OverMinutes   int    `json:"overMinutes"`
}

// TimeReceiptView is spec section 60's "TIME RECEIPT" end-of-day
// summary. Categories mirrors AnalyticsService.GetCategoryBreakdown's
// own CategoryTotal (id + seconds only - the frontend joins names from
// the category list it already has loaded, same convention as every
// other breakdown in this codebase).
type TimeReceiptView struct {
	DateLabel           string                  `json:"dateLabel"` // e.g. "August 16, 2026"
	Categories          []CategoryTotal         `json:"categories"`
	TotalTrackedSeconds int64                   `json:"totalTrackedSeconds"`
	Goals               []TimeReceiptGoalLine   `json:"goals"`
	Budgets             []TimeReceiptBudgetLine `json:"budgets"`
	// AINote is Phase 11's OPTIONAL commentary (spec section 37's
	// "daily reviews" use case, gated on ScopeDailyReviews) - see
	// aiNoteForReceipt's doc comment for the exact contract, identical
	// in spirit to IdeaResult.AINote in idea_service.go.
	AINote string `json:"aiNote,omitempty"`
}

// TimeReceiptService computes the Daily Time Receipt from real session
// data plus the existing Goal/Budget services - it deliberately holds
// no data of its own and never duplicates GoalService/BudgetService's
// period math (internal/periods), so "today's goal status" here and on
// the Goals/Budgets pages can never quietly drift apart.
type TimeReceiptService struct {
	sessionRepo   sessions.Repository
	goalService   *GoalService
	budgetService *BudgetService
	aiSvc         *ai.Service // always non-nil in practice (see idea_service.go's identical convention); nil is handled defensively for callers/tests that don't wire one
}

func NewTimeReceiptService(sessionRepo sessions.Repository, goalService *GoalService, budgetService *BudgetService, aiSvc *ai.Service) *TimeReceiptService {
	return &TimeReceiptService{sessionRepo: sessionRepo, goalService: goalService, budgetService: budgetService, aiSvc: aiSvc}
}

// GetTimeReceipt returns the receipt for the calendar day containing
// atUnix, in the local timezone (section 70) - or today, if atUnix is
// 0. Goal/budget lookups are best-effort: a failure there degrades to
// an empty section rather than failing the whole receipt, since the
// tracked-time numbers above them are the part of this screen a person
// is least able to get anywhere else and shouldn't be held hostage to
// an unrelated query failing.
func (s *TimeReceiptService) GetTimeReceipt(atUnix int64) (TimeReceiptView, error) {
	ctx := context.Background()

	at := time.Now()
	if atUnix > 0 {
		at = time.Unix(atUnix, 0)
	}
	start, end := periods.Window("daily", at)

	sums, err := s.sessionRepo.SumDurationByCategory(ctx, start, end)
	if err != nil {
		return TimeReceiptView{}, err
	}
	categories := make([]CategoryTotal, 0, len(sums))
	var total int64
	for id, seconds := range sums {
		categories = append(categories, CategoryTotal{CategoryID: id, Seconds: seconds})
		total += seconds
	}

	// Initialized empty, never nil: encoding/json marshals a nil slice
	// as `null`, and the frontend reads .length/.map off these fields
	// directly - a fresh database with no goals/budgets crashed the Time
	// Receipt page (and, via its persisted hash route, the whole app on
	// every reload) until this was fixed. Same rule everywhere a view
	// carries a slice: make(), never var.
	goalLines := make([]TimeReceiptGoalLine, 0)
	if s.goalService != nil {
		if statuses, err := s.goalService.ListWithStatus(); err == nil {
			for _, g := range statuses {
				goalLines = append(goalLines, TimeReceiptGoalLine{
					Name:            g.Name,
					Met:             g.PercentComplete >= 100,
					PercentComplete: g.PercentComplete,
				})
			}
		}
	}

	budgetLines := make([]TimeReceiptBudgetLine, 0)
	if s.budgetService != nil {
		if statuses, err := s.budgetService.ListWithStatus(); err == nil {
			for _, b := range statuses {
				if b.Schedule != "daily" && b.Schedule != "day" {
					continue // see TimeReceiptBudgetLine's doc comment
				}
				budgetLines = append(budgetLines, TimeReceiptBudgetLine{
					Name:          b.Name,
					LimitMinutes:  b.LimitMinutes,
					ActualMinutes: b.ActualMinutes,
					OverMinutes:   b.ActualMinutes - b.LimitMinutes,
				})
			}
		}
	}

	view := TimeReceiptView{
		DateLabel:           at.Format("January 2, 2006"),
		Categories:          categories,
		TotalTrackedSeconds: total,
		Goals:               goalLines,
		Budgets:             budgetLines,
	}
	view.AINote = s.aiNoteForReceipt(ctx, view)
	return view, nil
}

// aiNoteForReceipt mirrors idea_service.go's aiNoteForSuggestions
// contract exactly (see that function's doc comment for the full
// rationale): built only from numbers already on the receipt above,
// explicitly instructed not to invent facts or moralize (spec section
// 14: "Do not shame the user. Do not produce fake motivational quotes.
// Be factual."), and silently omitted on any failure.
func (s *TimeReceiptService) aiNoteForReceipt(ctx context.Context, view TimeReceiptView) string {
	if s.aiSvc == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Date: %s\n", view.DateLabel)
	for _, c := range view.Categories {
		fmt.Fprintf(&b, "Category %d: %dm\n", c.CategoryID, c.Seconds/60)
	}
	fmt.Fprintf(&b, "Total tracked: %dm\n", view.TotalTrackedSeconds/60)
	for _, g := range view.Goals {
		status := "not yet met"
		if g.Met {
			status = "met"
		}
		fmt.Fprintf(&b, "Goal %q: %s\n", g.Name, status)
	}
	for _, bl := range view.Budgets {
		fmt.Fprintf(&b, "Budget %q: %dm over (negative means under)\n", bl.Name, bl.OverMinutes)
	}

	result, err := s.aiSvc.Enhance(ctx, ai.ScopeDailyReviews, ai.Prompt{
		SystemPrompt: "You are an optional commentary add-on inside a personal time-tracking " +
			"desktop app called STELLAREI, on its end-of-day 'Time Receipt' screen. You will be " +
			"given the day's real numbers below. Add ONE short, plain, strictly factual sentence " +
			" - never a moral judgment, never fake encouragement, never an invented statistic not " +
			"in the numbers given. If nothing notable stands out, it is fine to say so plainly. " +
			"Do not use exclamation points or emoji. Reply with only that one sentence.",
		UserPrompt: b.String(),
		MaxTokens:  120,
	})
	if err != nil || result.Unavailable {
		return ""
	}
	return strings.TrimSpace(result.Text)
}
