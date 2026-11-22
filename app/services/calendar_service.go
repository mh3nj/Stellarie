package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"stellarei/internal/calendar"
	"stellarei/internal/sessions"
)

// CalendarDayView is one day's compact time summary - spec section
// 30's own example: "16 / Dev 4h / Design 2h / Gaming 1h". Categories
// mirrors AnalyticsService's own CategoryTotal (id + seconds only - the
// frontend joins names from the category list it already has loaded,
// same convention as every other breakdown in this codebase).
type CalendarDayView struct {
	Date         string          `json:"date"` // "2026-08-16"
	Categories   []CategoryTotal `json:"categories"`
	TotalSeconds int64           `json:"totalSeconds"`
}

// CalendarMonthView is a full month - only days with any tracked time
// appear in Days (spec section 67: never invent data for an empty day;
// the frontend renders an absent date as an empty calendar cell).
type CalendarMonthView struct {
	Year  int               `json:"year"`
	Month int               `json:"month"` // 1-12
	Days  []CalendarDayView `json:"days"`
}

// CalendarService computes the Calendar view (spec section 30) from
// real session data. Holds no state of its own - every call recomputes
// from current data, the same choice every other read-only analytics
// service in this codebase makes.
type CalendarService struct {
	sessionRepo sessions.Repository
}

func NewCalendarService(sessionRepo sessions.Repository) *CalendarService {
	return &CalendarService{sessionRepo: sessionRepo}
}

// GetMonth returns the compact per-day breakdown for the given calendar
// month, in the local timezone (spec section 70). month is 1-12.
func (s *CalendarService) GetMonth(year, month int) (CalendarMonthView, error) {
	if month < 1 || month > 12 {
		return CalendarMonthView{}, fmt.Errorf("calendar: invalid month %d", month)
	}
	ctx := context.Background()
	loc := time.Local

	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	views, err := s.sessionRepo.ListForRange(ctx, monthStart.Unix(), monthEnd.Unix())
	if err != nil {
		return CalendarMonthView{}, fmt.Errorf("calendar: loading sessions: %w", err)
	}
	samples := make([]calendar.SessionSample, 0, len(views))
	for _, v := range views {
		if v.Excluded {
			continue // spec section 13: excluded sessions never feed analytics of any kind
		}
		samples = append(samples, calendar.SessionSample{
			CategoryID:   v.CategoryID,
			CategoryName: v.CategoryName,
			StartedAt:    v.StartedAt,
			EndedAt:      v.EndedAt,
		})
	}

	dayTotals := calendar.DailyCategoryTotals(samples, loc)

	// ListForRange can return a session that starts before monthStart or
	// ends after monthEnd (anything OVERLAPPING the range) - a session
	// spanning the last night of this month into the first day of next
	// month, for instance. DailyCategoryTotals correctly splits that
	// session's duration across both real calendar days, but GetMonth
	// for THIS month must only report the days that actually fall
	// inside it - otherwise a Calendar page for August could show a
	// stray September 1st cell. monthStartKey/monthEndKey bound that
	// filter using the same "YYYY-MM-DD" string format
	// DailyCategoryTotals itself produces, so this is a plain string
	// comparison, not a second date-parsing step.
	monthStartKey := monthStart.Format("2006-01-02")
	monthEndKey := monthEnd.Format("2006-01-02") // exclusive

	dateKeys := make([]string, 0, len(dayTotals))
	for k := range dayTotals {
		if k >= monthStartKey && k < monthEndKey {
			dateKeys = append(dateKeys, k)
		}
	}
	sort.Strings(dateKeys)

	days := make([]CalendarDayView, 0, len(dateKeys))
	for _, dateKey := range dateKeys {
		totals := dayTotals[dateKey]
		catIDs := make([]int64, 0, len(totals))
		for id := range totals {
			catIDs = append(catIDs, id)
		}
		sort.Slice(catIDs, func(i, j int) bool { return catIDs[i] < catIDs[j] })

		cats := make([]CategoryTotal, 0, len(totals))
		var dayTotal int64
		for _, id := range catIDs {
			cats = append(cats, CategoryTotal{CategoryID: id, Seconds: totals[id]})
			dayTotal += totals[id]
		}
		days = append(days, CalendarDayView{Date: dateKey, Categories: cats, TotalSeconds: dayTotal})
	}

	return CalendarMonthView{Year: year, Month: month, Days: days}, nil
}
