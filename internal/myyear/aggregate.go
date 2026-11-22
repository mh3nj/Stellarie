// Package myyear implements the genuinely non-trivial, pure pieces of
// "MY YEAR" (spec section 61): ranking categories/projects by time,
// picking the longest sessions, and finding the busiest month. The rest
// of MY YEAR (abandoned-project classification, goal status, year-over-
// year comparison) lives directly in app/services/myyear_service.go
// because it inherently composes several repositories' data rather than
// transforming an already-fetched slice - this package holds only the
// parts that are pure functions of data the caller already has, the
// same decoupling convention internal/patterns/internal/waste/
// internal/calendar/internal/search each independently follow.
package myyear

import (
	"sort"
	"time"

	"stellarei/internal/calendar"
)

// SessionSample is the minimal per-session data this package needs.
type SessionSample struct {
	SessionID    int64
	CategoryID   int64
	CategoryName string
	ProjectID    int64
	ProjectName  string
	StartedAt    int64
	EndedAt      int64
}

func (s SessionSample) DurationSeconds() int64 {
	if s.EndedAt <= s.StartedAt {
		return 0
	}
	return s.EndedAt - s.StartedAt
}

// Total is one category or project's total time, in seconds.
type Total struct {
	ID      int64
	Seconds int64
}

// RankByCategory sums duration per category (skipping uncategorized,
// id 0) and returns them sorted by seconds descending - highest first,
// so the caller can cap to a top-N without a second sort.
func RankByCategory(samples []SessionSample) []Total {
	totals := map[int64]int64{}
	for _, s := range samples {
		if s.CategoryID == 0 {
			continue
		}
		totals[s.CategoryID] += s.DurationSeconds()
	}
	return sortedTotals(totals)
}

// RankByProject sums duration per project (skipping unassigned, id 0)
// and returns them sorted by seconds descending.
func RankByProject(samples []SessionSample) []Total {
	totals := map[int64]int64{}
	for _, s := range samples {
		if s.ProjectID == 0 {
			continue
		}
		totals[s.ProjectID] += s.DurationSeconds()
	}
	return sortedTotals(totals)
}

func sortedTotals(totals map[int64]int64) []Total {
	out := make([]Total, 0, len(totals))
	for id, secs := range totals {
		out = append(out, Total{ID: id, Seconds: secs})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		return out[i].ID < out[j].ID // stable tie-break, not map iteration order
	})
	return out
}

// LongestSession is one of the year's longest sessions, ready to
// display - Title prefers the project name, falling back to category,
// then a fixed "Uncategorized" label, matching every other
// title-resolution in this codebase (e.g. TimeReceiptService's own
// category-then-fallback convention).
type LongestSession struct {
	SessionID       int64
	Title           string
	DateKey         string // "YYYY-MM-DD", in the given location
	DurationMinutes int
}

// LongestSessions returns the top `limit` samples by duration,
// descending. Never mutates the caller's slice.
func LongestSessions(samples []SessionSample, limit int, loc *time.Location) []LongestSession {
	sorted := make([]SessionSample, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].DurationSeconds() > sorted[j].DurationSeconds() })
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	out := make([]LongestSession, 0, len(sorted))
	for _, s := range sorted {
		title := s.ProjectName
		if title == "" {
			title = s.CategoryName
		}
		if title == "" {
			title = "Uncategorized"
		}
		out = append(out, LongestSession{
			SessionID:       s.SessionID,
			Title:           title,
			DateKey:         time.Unix(s.StartedAt, 0).In(loc).Format("2006-01-02"),
			DurationMinutes: int(s.DurationSeconds() / 60),
		})
	}
	return out
}

// BusiestMonth finds the calendar month with the most tracked time,
// reusing internal/calendar.DailyCategoryTotals - already correct
// about splitting a session across a local-midnight boundary - and
// rolling per-day totals up by month prefix. A day total, once
// computed, belongs entirely to exactly one month (a day never itself
// spans two months), so this roll-up needs no boundary logic of its
// own beyond calendar's. Returns ("", 0) if samples has no data at all.
func BusiestMonth(samples []calendar.SessionSample, loc *time.Location) (label string, totalSeconds int64) {
	dayTotals := calendar.DailyCategoryTotals(samples, loc)
	monthTotals := map[string]int64{}
	for dayKey, categories := range dayTotals {
		if len(dayKey) < 7 {
			continue // malformed key defensively skipped, never panics on a slice bound
		}
		monthKey := dayKey[:7] // "2026-08"
		for _, seconds := range categories {
			monthTotals[monthKey] += seconds
		}
	}

	var bestKey string
	var bestSeconds int64
	for k, v := range monthTotals {
		if v > bestSeconds || (v == bestSeconds && (bestKey == "" || k < bestKey)) {
			bestKey, bestSeconds = k, v
		}
	}
	if bestKey == "" {
		return "", 0
	}
	t, err := time.Parse("2006-01", bestKey)
	if err != nil {
		return "", 0
	}
	return t.Format("January"), bestSeconds
}
