// Package patterns implements Pattern Discovery (spec section 31): a
// plain, deterministic analytics engine over real session data
// "STELLAREI observed you usually start Development around 10:30",
// never "this causes that". Exactly like internal/idea (see that
// package's own doc comment), this package is entirely free of AI and
// takes plain input structs the caller assembles from
// internal/sessions, rather than importing internal/sessions directly
// so pattern-detection logic here stays pure, deterministic, and
// testable without a database, and Phase 11's optional AI layer
// (app/services/pattern_service.go) stays a strictly additive
// rephrasing/summary layer bolted on afterward, never a source of new
// patterns itself (spec section 66: "Never fabricate patterns").
//
// Every function in this package follows the same two rules, by
// construction rather than by convention the caller has to remember:
//
//  1. Never claim causation. Every Insight.Text uses observational
//     phrasing ("occur", "are longer", "increased") - never "because",
//     "causes", or "leads to" - matching spec section 31's own examples
//     word for word.
//  2. Never fabricate significance. Every function has a minimum sample
//     size (and, where a percentage is involved, a minimum difference)
//     below which it reports nothing at all rather than a pattern a
//     handful of data points can't actually support - spec section 66's
//     "if data is insufficient: not enough data yet" applied here as
//     "then simply don't produce that Insight", the same way
//     idea_service.go's aiNote helpers return "" rather than a weak
//     guess.
package patterns

import (
	"strconv"
	"time"
)

// SessionSample is the minimal per-session data pattern-detection needs.
// The caller (app/services/pattern_service.go) builds these from
// sessions.View - CategoryID 0 (uncategorized) is intentionally skipped
// by every function below, since "you usually start Uncategorized
// around 10:30" isn't an insight anyone asked for.
type SessionSample struct {
	CategoryID   int64
	CategoryName string
	StartedAt    int64 // unix seconds
	EndedAt      int64 // unix seconds
}

// DurationSeconds is EndedAt - StartedAt, clamped at 0 so a
// malformed/inverted sample (which should never happen upstream, but
// this package doesn't trust its input blindly) can't produce a
// negative duration that would silently corrupt an average.
func (s SessionSample) DurationSeconds() int64 {
	if s.EndedAt <= s.StartedAt {
		return 0
	}
	return s.EndedAt - s.StartedAt
}

// Insight is one discovered pattern. Kind identifies which function
// produced it (useful for the frontend to pick an icon, or for a future
// "don't show me this kind again" preference - not used by this package
// itself). Text is the complete, display-ready sentence; this package
// never returns a template plus data for the frontend to assemble,
// because the exact phrasing (observational, hedged, "STELLAREI
// observed") is the whole point and shouldn't be re-derivable
// incorrectly by a second implementation.
type Insight struct {
	Kind         string `json:"kind"`
	CategoryID   int64  `json:"categoryId,omitempty"`
	CategoryName string `json:"categoryName,omitempty"`
	Text         string `json:"text"`
}

// clockLabel formats a minute-of-day (0-1439) as 24-hour "HH:MM" - used
// instead of importing time.Time formatting per-call so callers passing
// a plain int (a median across many timestamps, which isn't itself a
// real instant) don't have to fabricate a fake time.Time just to format
// it.
func clockLabel(minuteOfDay int) string {
	h := minuteOfDay / 60
	m := minuteOfDay % 60
	return twoDigits(h) + ":" + twoDigits(m)
}

func twoDigits(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 2 {
		return "0" + s
	}
	return s
}

// minuteOfDay extracts hour*60+minute from a unix timestamp, in the
// given location - section 70: a person's "10:30" is their local
// 10:30, not a UTC instant reinterpreted.
func minuteOfDay(unixSeconds int64, loc *time.Location) int {
	t := time.Unix(unixSeconds, 0).In(loc)
	return t.Hour()*60 + t.Minute()
}
