package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stellarei/internal/ai"
	"stellarei/internal/patterns"
	"stellarei/internal/periods"
	"stellarei/internal/sessions"
)

// PatternInsightView is patterns.Insight shaped for the frontend - same
// field set, JSON tags already match, so this exists mainly to keep the
// service layer's DTO naming convention consistent with every other
// *View type in this package rather than exporting an internal/ type
// directly across the Wails boundary. InsightType is the natural key
// (see internal/patterns/repository.go's doc comment) the frontend
// passes back to Dismiss/Undismiss - not part of internal/patterns'
// own Insight, since only the service layer (which owns persistence)
// needs it.
type PatternInsightView struct {
	Kind         string `json:"kind"`
	CategoryID   int64  `json:"categoryId,omitempty"`
	CategoryName string `json:"categoryName,omitempty"`
	Text         string `json:"text"`
	InsightType  string `json:"insightType"`
}

// PatternsView is the full Pattern Discovery result (spec section 31).
// AINote follows the exact same OPTIONAL, clearly-labeled, opt-in
// contract as IdeaResult.AINote / TimeReceiptView.AINote - see
// aiNoteForPatterns' doc comment for why it can only ever SUMMARIZE the
// Insights already found, never add a pattern of its own.
type PatternsView struct {
	Insights []PatternInsightView `json:"insights"`
	AINote   string               `json:"aiNote,omitempty"`
}

// patternLookbackDays bounds how far back TypicalStartTimes/
// TimeOfDayLengthPatterns look for session samples. 30 days balances
// "enough data for a median/average to mean something" against "still
// reflects how the person uses their computer now, not six months ago".
const patternLookbackDays = 30

// PatternService composes real session data into Pattern Discovery
// insights. Computation itself is always live - never cached, the same
// choice BudgetService/GoalService make - but which insight_types are
// dismissed IS persisted (internal/patterns.Repository, backed by the
// pattern_insights table 0001 anticipated and 0006 completed), so a
// person's "don't show me this again" survives a restart.
type PatternService struct {
	sessionRepo sessions.Repository
	repo        *patterns.Repository
	aiSvc       *ai.Service // see idea_service.go's identical convention: always real in practice, nil handled defensively
}

func NewPatternService(sessionRepo sessions.Repository, repo *patterns.Repository, aiSvc *ai.Service) *PatternService {
	return &PatternService{sessionRepo: sessionRepo, repo: repo, aiSvc: aiSvc}
}

// deriveInsightType derives the natural key Dismiss/Undismiss/Sync operate
// on - "<kind>:<categoryId>", e.g. "typical_start:3", or
// "context_switch_trend:0" for the one kind with no category. Kept as
// a single function so the service layer and nothing else defines this
// format - internal/patterns itself has no opinion on it (see
// PatternInsightView's doc comment).
func deriveInsightType(in patterns.Insight) string {
	return fmt.Sprintf("%s:%d", in.Kind, in.CategoryID)
}

// GetInsights runs Pattern Discovery over the last patternLookbackDays
// of session data plus this-week-vs-last-week context-switch stats.
// Goal/budget-style best-effort degradation applies here too: if the
// context-switch comparison can't be computed, TypicalStartTimes/
// TimeOfDayLengthPatterns insights are still returned - one failed
// sub-computation never blanks the whole page.
func (s *PatternService) GetInsights() (PatternsView, error) {
	ctx := context.Background()
	now := time.Now()

	lookbackStart := now.AddDate(0, 0, -patternLookbackDays).Unix()
	views, err := s.sessionRepo.ListForRange(ctx, lookbackStart, now.Unix())
	if err != nil {
		return PatternsView{}, fmt.Errorf("patterns: loading sessions: %w", err)
	}
	samples := make([]patterns.SessionSample, 0, len(views))
	for _, v := range views {
		if v.Excluded {
			continue // spec section 13: an excluded session must not feed analytics of any kind
		}
		samples = append(samples, patterns.SessionSample{
			CategoryID:   v.CategoryID,
			CategoryName: v.CategoryName,
			StartedAt:    v.StartedAt,
			EndedAt:      v.EndedAt,
		})
	}

	// This-week-vs-last-week context-switch comparison, reusing the
	// exact same window math and switch-counting logic Analytics/Budgets
	// already use (periods.Window, sessionRepo.ContextSwitchStats)
	// never a second, possibly-inconsistent implementation.
	thisStart, thisEnd := periods.Window("weekly", now)
	lastStart, lastEnd := thisStart-(thisEnd-thisStart), thisStart
	var thisWeekAvg, lastWeekAvg int64
	var thisWeekSwitches int
	if stats, err := s.sessionRepo.ContextSwitchStats(ctx, thisStart, thisEnd); err == nil {
		thisWeekAvg, thisWeekSwitches = stats.AverageSeconds, stats.Switches
	}
	if stats, err := s.sessionRepo.ContextSwitchStats(ctx, lastStart, lastEnd); err == nil {
		lastWeekAvg = stats.AverageSeconds
	}

	insights := patterns.DiscoverAll(samples, time.Local, thisWeekAvg, lastWeekAvg, thisWeekSwitches)

	// Best-effort, exactly like the goal/budget lookups above: a
	// persistence hiccup degrades to "nothing dismissed, sync skipped"
	// rather than blanking the whole page - the live-computed insights
	// are still the part of this screen a person can least afford to
	// lose to an unrelated failure.
	dismissed, _ := s.repo.DismissedTypes(ctx)

	views2 := make([]PatternInsightView, 0, len(insights))
	for _, in := range insights {
		t := deriveInsightType(in)
		if s.repo != nil {
			_ = s.repo.Sync(ctx, t, in.Text) // best-effort; see comment above
		}
		if dismissed[t] {
			continue
		}
		views2 = append(views2, PatternInsightView{Kind: in.Kind, CategoryID: in.CategoryID, CategoryName: in.CategoryName, Text: in.Text, InsightType: t})
	}

	view := PatternsView{Insights: views2}
	view.AINote = s.aiNoteForPatterns(ctx, insights)
	return view, nil
}

// Dismiss hides one insight going forward (spec's own pattern_insights
// schema anticipated this - see 0006's migration comment). insightType
// is the value PatternInsightView.InsightType supplied for that
// insight; the frontend never needs to know or construct the "<kind>:
// <categoryId>" format itself.
func (s *PatternService) Dismiss(insightType string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Dismiss(context.Background(), insightType)
}

// Undismiss reverses Dismiss - the next GetInsights call shows it again
// with current data, not the wording from when it was dismissed.
func (s *PatternService) Undismiss(insightType string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Undismiss(context.Background(), insightType)
}

// aiNoteForPatterns mirrors idea_service.go's aiNoteForSuggestions /
// receipt_service.go's aiNoteForReceipt contract exactly, with one
// addition specific to this feature: the system prompt explicitly
// forbids introducing any pattern not already in the list, because spec
// section 66 says "never fabricate patterns" and an LLM asked to
// "comment on someone's habits" will readily invent a plausible-sounding
// one if not stopped. This function can only ever SUMMARIZE or connect
// insights patterns.DiscoverAll already found through real, tested,
// deterministic math - never add one of its own.
func (s *PatternService) aiNoteForPatterns(ctx context.Context, insights []patterns.Insight) string {
	if s.aiSvc == nil || len(insights) == 0 {
		return ""
	}
	var b strings.Builder
	for i, in := range insights {
		fmt.Fprintf(&b, "%d. %s\n", i+1, in.Text)
	}

	result, err := s.aiSvc.Enhance(ctx, ai.ScopePatterns, ai.Prompt{
		SystemPrompt: "You are an optional commentary add-on inside a personal time-tracking " +
			"desktop app called STELLAREI, on its Pattern Discovery screen. You will be given a " +
			"numbered list of patterns already found by the app's own deterministic analysis. Add " +
			"ONE short, plain sentence connecting or summarizing them. You must NOT introduce any " +
			"pattern, statistic, or observation that is not already in the list given to you, and " +
			"you must never claim one thing causes another - only describe what was observed, " +
			"exactly like the list itself does. Do not use exclamation points or emoji. Reply with " +
			"only that one sentence.",
		UserPrompt: b.String(),
		MaxTokens:  120,
	})
	if err != nil || result.Unavailable {
		return ""
	}
	return strings.TrimSpace(result.Text)
}
