// Package sessions turns a stream of already-persisted activity events
// into sessions (spec section 8) - the user-facing "09:32-10:48 Serenity
// Development, 1h 16m" view, instead of raw per-app-switch rows.
//
// Phase 2 scope: sessionization here only considers application
// continuity and idle gaps, because project association (section 11) and
// category assignment (section 65's rule engine) are explicitly Phase 3
// and Phase 4 work (spec section 84) - there's no project_id/category_id
// to group by yet. Every BuiltSession's ProjectID/CategoryID stays unset;
// Phase 3/4 extend this package rather than replace it.
package sessions

import "time"

// RawEvent is the subset of an activity_events row the sessionizer needs.
// ApplicationID of 0 is treated as "unknown application" and never
// merged with another event, matching the low confidence such rows
// should carry.
type RawEvent struct {
	ID            int64
	StartedAt     time.Time
	Duration      time.Duration
	ApplicationID int64
	ProcessName   string // for project-rule matching (section 11/65) - not persisted on activity_events itself, just threaded through
	WindowTitle   string
	Idle          bool
	Confidence    float64
}

// EndedAt is a convenience derived from StartedAt+Duration.
func (e RawEvent) EndedAt() time.Time {
	return e.StartedAt.Add(e.Duration)
}

// BuiltSession is a closed, ready-to-persist session.
type BuiltSession struct {
	StartedAt     time.Time
	EndedAt       time.Time
	ApplicationID int64
	ProcessName   string  // most recently observed within the session - what project-rule evaluation runs against
	WindowTitle   string  // most recently observed within the session
	ProjectID     int64   // 0 until a resolver (spec section 65) assigns one - see activity.Engine's ProjectResolver
	CategoryID    int64   // 0 until a resolver assigns one - see activity.Engine's CategoryResolver (section 10)
	Confidence    float64 // the minimum confidence of any member event
	EventIDs      []int64
}

// maxMergeGap is how close two same-application events must be to count
// as one continuous session rather than two adjacent ones. This exists
// because the Activity Engine (internal/activity) silently drops
// sub-1-second events as poll noise - without a merge tolerance, a
// filtered-out half-second glance at another window would incorrectly
// fragment an otherwise continuous session.
const maxMergeGap = 5 * time.Second

// IncrementalSessionizer processes events one at a time as the Activity
// Engine persists them, closing out and returning a BuiltSession exactly
// when a session boundary is crossed (idle, app switch beyond
// maxMergeGap, or an explicit Flush on shutdown). Mirrors
// activity.Debouncer's shape deliberately - same "feed one at a time,
// get a completed unit back or nil" contract, same "not safe for
// concurrent use, owned by one goroutine" rule.
type IncrementalSessionizer struct {
	current *BuiltSession
}

func NewIncrementalSessionizer() *IncrementalSessionizer {
	return &IncrementalSessionizer{}
}

// Feed processes one raw event in chronological order.
func (s *IncrementalSessionizer) Feed(ev RawEvent) *BuiltSession {
	if ev.Idle {
		// Idle time is never part of a session - it always closes
		// whatever was open. section 9: "do not count idle time as
		// productive activity."
		return s.closeCurrent()
	}

	if s.current != nil &&
		s.current.ApplicationID == ev.ApplicationID &&
		ev.ApplicationID != 0 &&
		!ev.StartedAt.After(s.current.EndedAt.Add(maxMergeGap)) {
		s.current.EndedAt = ev.EndedAt()
		s.current.WindowTitle = ev.WindowTitle // most recent observed title wins, same convention as activity.Debouncer
		if ev.Confidence < s.current.Confidence {
			s.current.Confidence = ev.Confidence
		}
		s.current.EventIDs = append(s.current.EventIDs, ev.ID)
		return nil
	}

	closed := s.closeCurrent()
	s.current = &BuiltSession{
		StartedAt:     ev.StartedAt,
		EndedAt:       ev.EndedAt(),
		ApplicationID: ev.ApplicationID,
		ProcessName:   ev.ProcessName,
		WindowTitle:   ev.WindowTitle,
		Confidence:    ev.Confidence,
		EventIDs:      []int64{ev.ID},
	}
	return closed
}

// Flush closes whatever session is in progress, for use on shutdown or
// pause. Returns nil if nothing was open.
func (s *IncrementalSessionizer) Flush() *BuiltSession {
	return s.closeCurrent()
}

func (s *IncrementalSessionizer) closeCurrent() *BuiltSession {
	if s.current == nil {
		return nil
	}
	closed := s.current
	s.current = nil
	return closed
}

// BuildSessions runs a full slice of chronologically-ordered events
// through a fresh IncrementalSessionizer and returns every session that
// results, flushing at the end. Useful for backfilling/rebuilding
// sessions from raw events (e.g. after a manual correction changes which
// events exist) without duplicating the merge logic above.
func BuildSessions(events []RawEvent) []BuiltSession {
	s := NewIncrementalSessionizer()
	var out []BuiltSession
	for _, ev := range events {
		if built := s.Feed(ev); built != nil {
			out = append(out, *built)
		}
	}
	if built := s.Flush(); built != nil {
		out = append(out, *built)
	}
	return out
}
