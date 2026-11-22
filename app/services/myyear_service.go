package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"stellarei/internal/ai"
	"stellarei/internal/calendar"
	"stellarei/internal/myyear"
	"stellarei/internal/projects"
	"stellarei/internal/sessions"
)

// abandonedThresholdDays and abandonedMinHistoricalMinutes bound the
// INFERRED half of MyYearView.AbandonedProjects (see its own doc
// comment) - a project needs BOTH no activity in a long while AND real
// historical time invested before it's worth surfacing as
// possibly-abandoned; a project touched once for five minutes going
// quiet isn't evidence of anything.
const (
	abandonedThresholdDays        = 90
	abandonedMinHistoricalMinutes = 60
)

// maxTopProjects and maxLongestSessions cap the corresponding lists
// spec's own example presentation implies a short, scannable list, not
// every project/session from a full year.
const (
	maxTopProjects     = 10
	maxLongestSessions = 5
)

// MyYearAbandonedProject is one project MyYear surfaces as possibly (or
// explicitly) abandoned. Explicit means the person themselves set the
// project's Status to "abandoned" - a real, direct signal. Inferred
// means the project is still marked "active" but hasn't been touched in
// abandonedThresholdDays+ despite real historical time invested - an
// evidence-only observation, not a verdict (same discipline as
// internal/waste - see that package's doc comment for why this
// matters).
type MyYearAbandonedProject struct {
	Name              string `json:"name"`
	Explicit          bool   `json:"explicit"`
	DaysSinceActivity int    `json:"daysSinceActivity"`
	HistoricalMinutes int    `json:"historicalMinutes"`
}

// MyYearLongestSession is one of the year's longest sessions.
type MyYearLongestSession struct {
	Title           string `json:"title"` // project or category name, whichever is more specific
	DateKey         string `json:"dateKey"`
	DurationMinutes int    `json:"durationMinutes"`
}

// MyYearView is the full "MY YEAR" overview (spec section 61).
// GoalsCurrentlyComplete is deliberately named for what it actually is
// - CURRENT completion status, not "completed during this specific
// year" - because GoalStatus has no completion timestamp to determine
// the latter honestly (same caveat TimeReceiptView.Goals already
// states for the identical reason).
type MyYearView struct {
	Year                   int                      `json:"year"`
	TotalTrackedSeconds    int64                    `json:"totalTrackedSeconds"`
	Categories             []CategoryTotal          `json:"categories"`
	TopProjects            []ProjectTotal           `json:"topProjects"`
	LongestSessions        []MyYearLongestSession   `json:"longestSessions"`
	BusiestMonthLabel      string                   `json:"busiestMonthLabel"` // "August", "" if no data
	BusiestMonthSeconds    int64                    `json:"busiestMonthSeconds"`
	AbandonedProjects      []MyYearAbandonedProject `json:"abandonedProjects"`
	GoalsCurrentlyComplete []string                 `json:"goalsCurrentlyComplete"`
	HasPriorYearData       bool                     `json:"hasPriorYearData"`
	PriorYearTotalSeconds  int64                    `json:"priorYearTotalSeconds"`
	AINote                 string                   `json:"aiNote,omitempty"`
}

// MyYearService composes MY YEAR from data every other analytics
// service already has a way to compute - this file's own job is
// specifically the three things that DON'T already exist elsewhere:
// picking the longest sessions, finding the busiest month (by reusing
// internal/calendar's already-correct day-bucketing rather than
// reimplementing month-boundary logic), and classifying abandoned
// projects.
type MyYearService struct {
	sessionRepo sessions.Repository
	projectRepo *projects.Repository
	goalService *GoalService
	aiSvc       *ai.Service
}

func NewMyYearService(sessionRepo sessions.Repository, projectRepo *projects.Repository, goalService *GoalService, aiSvc *ai.Service) *MyYearService {
	return &MyYearService{sessionRepo: sessionRepo, projectRepo: projectRepo, goalService: goalService, aiSvc: aiSvc}
}

// GetYear returns the MY YEAR overview for the given calendar year, in
// the local timezone (spec section 70).
func (s *MyYearService) GetYear(year int) (MyYearView, error) {
	ctx := context.Background()
	loc := time.Local

	yearStart := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	yearEnd := yearStart.AddDate(1, 0, 0)

	views, err := s.sessionRepo.ListForRange(ctx, yearStart.Unix(), yearEnd.Unix())
	if err != nil {
		return MyYearView{}, fmt.Errorf("myyear: loading sessions: %w", err)
	}
	var included []sessions.View
	for _, v := range views {
		if !v.Excluded {
			included = append(included, v)
		}
	}

	view := MyYearView{Year: year}

	mySamples := make([]myyear.SessionSample, 0, len(included))
	for _, v := range included {
		if v.EndedAt-v.StartedAt <= 0 {
			continue
		}
		view.TotalTrackedSeconds += v.EndedAt - v.StartedAt
		mySamples = append(mySamples, myyear.SessionSample{
			SessionID: v.ID, CategoryID: v.CategoryID, CategoryName: v.CategoryName,
			ProjectID: v.ProjectID, ProjectName: v.ProjectName, StartedAt: v.StartedAt, EndedAt: v.EndedAt,
		})
	}
	view.Categories = toCategoryTotalViews(myyear.RankByCategory(mySamples))
	view.TopProjects = toProjectTotalViews(myyear.RankByProject(mySamples), maxTopProjects)
	view.LongestSessions = toLongestSessionViews(myyear.LongestSessions(mySamples, maxLongestSessions, loc))

	calendarSamples := make([]calendar.SessionSample, 0, len(mySamples))
	for _, s := range mySamples {
		calendarSamples = append(calendarSamples, calendar.SessionSample{
			CategoryID: s.CategoryID, CategoryName: s.CategoryName, StartedAt: s.StartedAt, EndedAt: s.EndedAt,
		})
	}
	view.BusiestMonthLabel, view.BusiestMonthSeconds = myyear.BusiestMonth(calendarSamples, loc)

	if s.projectRepo != nil {
		view.AbandonedProjects = s.abandonedProjects(ctx, yearEnd)
	}
	// Same nil-slice rule as receipt_service.go: GoalsCurrentlyComplete
	// must be [] rather than null when nothing is complete yet.
	view.GoalsCurrentlyComplete = make([]string, 0)
	if s.goalService != nil {
		if statuses, err := s.goalService.ListWithStatus(); err == nil {
			for _, g := range statuses {
				if g.PercentComplete >= 100 {
					view.GoalsCurrentlyComplete = append(view.GoalsCurrentlyComplete, g.Name)
				}
			}
		}
	}

	priorStart := yearStart.AddDate(-1, 0, 0)
	if priorViews, err := s.sessionRepo.ListForRange(ctx, priorStart.Unix(), yearStart.Unix()); err == nil {
		var priorTotal int64
		var any bool
		for _, v := range priorViews {
			if v.Excluded {
				continue
			}
			if dur := v.EndedAt - v.StartedAt; dur > 0 {
				priorTotal += dur
				any = true
			}
		}
		if any {
			view.HasPriorYearData = true
			view.PriorYearTotalSeconds = priorTotal
		}
	}

	view.AINote = s.aiNoteForYear(ctx, view)
	return view, nil
}

func toCategoryTotalViews(totals []myyear.Total) []CategoryTotal {
	out := make([]CategoryTotal, 0, len(totals))
	for _, t := range totals {
		out = append(out, CategoryTotal{CategoryID: t.ID, Seconds: t.Seconds})
	}
	return out
}

func toProjectTotalViews(totals []myyear.Total, limit int) []ProjectTotal {
	if len(totals) > limit {
		totals = totals[:limit]
	}
	out := make([]ProjectTotal, 0, len(totals))
	for _, t := range totals {
		out = append(out, ProjectTotal{ProjectID: t.ID, Seconds: t.Seconds})
	}
	return out
}

func toLongestSessionViews(sessions []myyear.LongestSession) []MyYearLongestSession {
	out := make([]MyYearLongestSession, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, MyYearLongestSession{Title: s.Title, DateKey: s.DateKey, DurationMinutes: s.DurationMinutes})
	}
	return out
}

func (s *MyYearService) abandonedProjects(ctx context.Context, asOf time.Time) []MyYearAbandonedProject {
	out := make([]MyYearAbandonedProject, 0)
	all, err := s.projectRepo.ListProjects(ctx, true)
	if err != nil {
		return out
	}
	lastActivity, err := s.sessionRepo.LastActivityByProject(ctx)
	if err != nil {
		lastActivity = map[int64]int64{}
	}
	// Historical time needs the full history, not just this year - a
	// project abandoned mid-year could have most of its real investment
	// in a PRIOR year, and this must still count it.
	allTimeTotals, _ := s.sessionRepo.SumDurationByProject(ctx, 0, asOf.Unix())

	for _, p := range all {
		historicalMinutes := int(allTimeTotals[p.ID] / 60)
		if p.Status == "abandoned" {
			days := 0
			if touched, ok := lastActivity[p.ID]; ok {
				days = int(asOf.Sub(time.Unix(touched, 0)).Hours() / 24)
			}
			out = append(out, MyYearAbandonedProject{Name: p.Name, Explicit: true, DaysSinceActivity: days, HistoricalMinutes: historicalMinutes})
			continue
		}
		if p.Status != "active" {
			continue
		}
		touched, ok := lastActivity[p.ID]
		if !ok {
			continue // never touched at all isn't evidence of abandonment - no baseline to compare against
		}
		days := int(asOf.Sub(time.Unix(touched, 0)).Hours() / 24)
		if days >= abandonedThresholdDays && historicalMinutes >= abandonedMinHistoricalMinutes {
			out = append(out, MyYearAbandonedProject{Name: p.Name, Explicit: false, DaysSinceActivity: days, HistoricalMinutes: historicalMinutes})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysSinceActivity > out[j].DaysSinceActivity })
	return out
}

// aiNoteForYear mirrors every other aiNote function's contract in this
// codebase (see idea_service.go's aiNoteForSuggestions for the
// canonical doc comment): built only from the numbers already on the
// view, never inventing a fact not given. Gated on ScopeSummaries
// the one scope that had been defined since Phase 11's foundation but
// never actually wired to a caller until this feature (an honest
// correction to this project's own earlier "six of six" claim - see
// README's Phase 11 section for the full account).
func (s *MyYearService) aiNoteForYear(ctx context.Context, view MyYearView) string {
	if s.aiSvc == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Year: %d\n", view.Year)
	fmt.Fprintf(&b, "Total tracked: %dh\n", view.TotalTrackedSeconds/3600)
	if view.BusiestMonthLabel != "" {
		fmt.Fprintf(&b, "Busiest month: %s\n", view.BusiestMonthLabel)
	}
	if view.HasPriorYearData {
		fmt.Fprintf(&b, "Prior year total: %dh\n", view.PriorYearTotalSeconds/3600)
	}
	for _, c := range view.Categories {
		fmt.Fprintf(&b, "Category %d: %dh\n", c.CategoryID, c.Seconds/3600)
	}
	for _, ls := range view.LongestSessions {
		fmt.Fprintf(&b, "Longest session: %s, %dm\n", ls.Title, ls.DurationMinutes)
	}
	for _, ap := range view.AbandonedProjects {
		fmt.Fprintf(&b, "Abandoned project: %s (%d days)\n", ap.Name, ap.DaysSinceActivity)
	}

	result, err := s.aiSvc.Enhance(ctx, ai.ScopeSummaries, ai.Prompt{
		SystemPrompt: "You are an optional commentary add-on inside a personal time-tracking " +
			"desktop app called STELLAREI, on its 'MY YEAR' end-of-year overview. You will be " +
			"given the year's real numbers below. Write ONE short, plain, strictly factual " +
			"sentence highlighting something notable - never a moral judgment, never fake " +
			"encouragement, never a statistic not in the numbers given. Do not use exclamation " +
			"points or emoji. Reply with only that one sentence.",
		UserPrompt: b.String(),
		MaxTokens:  120,
	})
	if err != nil || result.Unavailable {
		return ""
	}
	return strings.TrimSpace(result.Text)
}
