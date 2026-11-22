// Package calendar implements the Calendar view (spec section 30):
// "a calendar where every day shows a compact time summary." Entirely
// AI-free, decoupled from internal/sessions the same way
// internal/idea/internal/patterns/internal/waste each independently
// are (see their package docs) - the caller
// (app/services/calendar_service.go) builds SessionSample values from
// sessions.View.
//
// The one genuinely non-trivial piece of logic here: a session that
// starts before local midnight and ends after it belongs, in duration
// terms, to BOTH calendar days - spec's own example ("16 / Dev 4h")
// implies each day's number is real tracked time on THAT day, not
// "whichever day the session happened to start on." A session doesn't
// have to literally run overnight to matter here - even one that
// starts at 23:58 and ends at 00:15 should show 2 minutes on the first
// day and 15 on the second, not all 17 minutes dumped on whichever day
// its StartedAt timestamp happens to fall on. SplitByLocalDay handles
// this by clamping each session's contribution to each day boundary it
// touches, rather than bucketing by the session's start time alone.
package calendar

import "time"

// SessionSample is the minimal per-session data this package needs.
type SessionSample struct {
	CategoryID   int64
	CategoryName string
	StartedAt    int64 // unix seconds
	EndedAt      int64 // unix seconds
}

// DayTotals is one calendar day's category breakdown, keyed by
// category ID, in seconds.
type DayTotals map[int64]int64

// DailyCategoryTotals buckets samples into per-local-day category
// totals across [rangeStart, rangeEnd) - the caller (usually a
// calendar month) - correctly splitting any session that crosses a
// local-midnight boundary between the days it actually touches (see
// package doc). Days with zero tracked time are simply absent from the
// returned map - the frontend renders those as empty calendar cells
// rather than this package inventing a zero-filled entry for every
// single day in the range up front.
func DailyCategoryTotals(samples []SessionSample, loc *time.Location) map[string]DayTotals {
	out := map[string]DayTotals{}
	for _, s := range samples {
		if s.CategoryID == 0 || s.EndedAt <= s.StartedAt {
			continue
		}
		for _, part := range splitByLocalDay(s.StartedAt, s.EndedAt, loc) {
			day := part.dayKey
			if out[day] == nil {
				out[day] = DayTotals{}
			}
			out[day][s.CategoryID] += part.seconds
		}
	}
	return out
}

type dayPart struct {
	dayKey  string // "2026-08-16"
	seconds int64
}

// splitByLocalDay divides [startUnix, endUnix) into one dayPart per
// local calendar day it overlaps, each clamped to that day's actual
// boundaries. A session entirely within one day returns exactly one
// part with its full duration - the common case stays cheap (one loop
// iteration, no allocation beyond the single-element slice).
func splitByLocalDay(startUnix, endUnix int64, loc *time.Location) []dayPart {
	start := time.Unix(startUnix, 0).In(loc)
	end := time.Unix(endUnix, 0).In(loc)

	var parts []dayPart
	cursor := start
	for cursor.Before(end) {
		dayStart := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), 0, 0, 0, 0, loc)
		dayEnd := dayStart.AddDate(0, 0, 1)

		segmentEnd := end
		if dayEnd.Before(segmentEnd) {
			segmentEnd = dayEnd
		}

		seconds := int64(segmentEnd.Sub(cursor).Seconds())
		if seconds > 0 {
			parts = append(parts, dayPart{dayKey: dayStart.Format("2006-01-02"), seconds: seconds})
		}
		cursor = segmentEnd
	}
	return parts
}
