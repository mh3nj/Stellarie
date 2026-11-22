package sessions

import (
	"testing"
	"time"
)

func at(seconds int) time.Time {
	return time.Date(2026, 8, 16, 9, 0, seconds, 0, time.UTC)
}

func ev(id int64, startSec, durationSec int, appID int64, idle bool) RawEvent {
	return RawEvent{
		ID:            id,
		StartedAt:     at(startSec),
		Duration:      time.Duration(durationSec) * time.Second,
		ApplicationID: appID,
		Idle:          idle,
		Confidence:    1.0,
	}
}

func TestIncrementalSessionizer_SingleEventNoFollowUp(t *testing.T) {
	s := NewIncrementalSessionizer()
	if got := s.Feed(ev(1, 0, 60, 10, false)); got != nil {
		t.Fatalf("Feed on first event = %+v, want nil (session still open)", got)
	}
	got := s.Flush()
	if got == nil {
		t.Fatal("Flush() = nil, want the open session closed")
	}
	if got.ApplicationID != 10 || len(got.EventIDs) != 1 || got.EventIDs[0] != 1 {
		t.Errorf("got %+v, want a single-event session for application 10", got)
	}
}

func TestIncrementalSessionizer_SameAppMergesIntoOneSession(t *testing.T) {
	s := NewIncrementalSessionizer()
	s.Feed(ev(1, 0, 60, 10, false))          // 09:00:00-09:01:00
	s.Feed(ev(2, 61, 300, 10, false))        // 09:01:01-09:06:01 (1s gap, within tolerance)
	got := s.Feed(ev(3, 362, 60, 10, false)) // 09:06:02, still same app -> no close yet
	if got != nil {
		t.Fatalf("Feed while still merging = %+v, want nil", got)
	}

	final := s.Flush()
	if final == nil {
		t.Fatal("Flush() = nil, want the merged session")
	}
	if len(final.EventIDs) != 3 {
		t.Errorf("EventIDs = %v, want all 3 events merged into one session", final.EventIDs)
	}
	wantEnd := at(362 + 60)
	if final.EndedAt != wantEnd {
		t.Errorf("EndedAt = %v, want %v", final.EndedAt, wantEnd)
	}
}

func TestIncrementalSessionizer_AppSwitchClosesSessionAndStartsNew(t *testing.T) {
	s := NewIncrementalSessionizer()
	s.Feed(ev(1, 0, 60, 10, false)) // application 10, 09:00:00-09:01:00

	closed := s.Feed(ev(2, 61, 120, 20, false)) // switches to application 20
	if closed == nil {
		t.Fatal("Feed() on app switch = nil, want the application-10 session closed")
	}
	if closed.ApplicationID != 10 || len(closed.EventIDs) != 1 {
		t.Errorf("closed session = %+v, want application 10 with 1 event", closed)
	}

	final := s.Flush()
	if final == nil || final.ApplicationID != 20 {
		t.Fatalf("Flush() = %+v, want the application-20 session", final)
	}
}

func TestIncrementalSessionizer_IdleAlwaysClosesTheSession(t *testing.T) {
	s := NewIncrementalSessionizer()
	s.Feed(ev(1, 0, 60, 10, false))

	closed := s.Feed(ev(2, 60, 900, 10, true)) // idle, same application ID
	if closed == nil {
		t.Fatal("Feed() on idle transition = nil, want the prior session closed")
	}
	if closed.ApplicationID != 10 {
		t.Errorf("closed.ApplicationID = %d, want 10", closed.ApplicationID)
	}

	// Idle events themselves never open a session - after this, nothing
	// should be in progress even if idle continues.
	if again := s.Feed(ev(3, 960, 60, 10, true)); again != nil {
		t.Fatalf("Feed() on continued idle = %+v, want nil", again)
	}
	if got := s.Flush(); got != nil {
		t.Fatalf("Flush() after only idle events = %+v, want nil (idle never opens a session)", got)
	}
}

func TestIncrementalSessionizer_GapBeyondToleranceStartsNewSessionEvenSameApp(t *testing.T) {
	s := NewIncrementalSessionizer()
	s.Feed(ev(1, 0, 60, 10, false)) // ends at t=60

	// Next same-app event starts at t=120 - a 60s gap, well beyond the
	// 5s merge tolerance. This models a real gap the debouncer's
	// noise-filtering wouldn't produce (e.g. an idle event was skipped
	// from this test's input) - the sessionizer must not silently bridge
	// a 60-second unaccounted-for gap.
	closed := s.Feed(ev(2, 120, 60, 10, false))
	if closed == nil {
		t.Fatal("Feed() with gap beyond tolerance = nil, want the first session closed")
	}
	if len(closed.EventIDs) != 1 {
		t.Errorf("closed session merged %d events, want exactly 1 (no bridging a 60s gap)", len(closed.EventIDs))
	}
}

func TestIncrementalSessionizer_GapWithinToleranceMerges(t *testing.T) {
	s := NewIncrementalSessionizer()
	s.Feed(ev(1, 0, 60, 10, false))         // ends at t=60
	got := s.Feed(ev(2, 64, 60, 10, false)) // starts at t=64, 4s gap - within the 5s tolerance
	if got != nil {
		t.Fatalf("Feed() with gap within tolerance = %+v, want nil (merged)", got)
	}
	final := s.Flush()
	if len(final.EventIDs) != 2 {
		t.Errorf("EventIDs = %v, want both events merged", final.EventIDs)
	}
}

func TestIncrementalSessionizer_UnknownApplicationNeverMerges(t *testing.T) {
	s := NewIncrementalSessionizer()
	s.Feed(ev(1, 0, 60, 0, false)) // ApplicationID 0 = unknown
	closed := s.Feed(ev(2, 61, 60, 0, false))
	if closed == nil {
		t.Fatal("Feed() with ApplicationID 0 twice in a row = nil, want each treated as its own session")
	}
	if len(closed.EventIDs) != 1 {
		t.Errorf("closed session merged %d unknown-application events, want 1 (never merge unknowns)", len(closed.EventIDs))
	}
}

func TestIncrementalSessionizer_ConfidenceTracksMinimumOfMergedEvents(t *testing.T) {
	s := NewIncrementalSessionizer()
	e1 := ev(1, 0, 60, 10, false)
	e1.Confidence = 0.95
	e2 := ev(2, 61, 60, 10, false)
	e2.Confidence = 0.60

	s.Feed(e1)
	s.Feed(e2)
	final := s.Flush()
	if final.Confidence != 0.60 {
		t.Errorf("Confidence = %v, want the minimum (0.60) across merged events", final.Confidence)
	}
}

func TestIncrementalSessionizer_TracksMostRecentWindowTitle(t *testing.T) {
	s := NewIncrementalSessionizer()
	e1 := ev(1, 0, 60, 10, false)
	e1.ProcessName = "Code.exe"
	e1.WindowTitle = "main.go"
	e2 := ev(2, 61, 60, 10, false)
	e2.ProcessName = "Code.exe"
	e2.WindowTitle = "debounce.go"

	s.Feed(e1)
	s.Feed(e2)
	final := s.Flush()
	if final.WindowTitle != "debounce.go" {
		t.Errorf("WindowTitle = %q, want the most recently observed title %q", final.WindowTitle, "debounce.go")
	}
	if final.ProcessName != "Code.exe" {
		t.Errorf("ProcessName = %q, want Code.exe", final.ProcessName)
	}
}

func TestBuildSessions_MatchesIncrementalResults(t *testing.T) {
	events := []RawEvent{
		ev(1, 0, 60, 10, false),
		ev(2, 61, 300, 10, false),   // merges with #1
		ev(3, 400, 900, 10, true),   // idle - closes the running session
		ev(4, 1300, 120, 20, false), // new session, application 20
	}

	got := BuildSessions(events)
	if len(got) != 2 {
		t.Fatalf("BuildSessions() returned %d sessions, want 2", len(got))
	}
	if got[0].ApplicationID != 10 || len(got[0].EventIDs) != 2 {
		t.Errorf("session[0] = %+v, want application 10 with 2 merged events", got[0])
	}
	if got[1].ApplicationID != 20 || len(got[1].EventIDs) != 1 {
		t.Errorf("session[1] = %+v, want application 20 with 1 event", got[1])
	}
}
