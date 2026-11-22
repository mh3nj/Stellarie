package periods

import (
	"testing"
	"time"
)

func TestWindow_Day(t *testing.T) {
	now := time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC) // Wednesday
	start, end := Window("daily", now)
	wantStart := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC).Unix()
	wantEnd := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC).Unix()
	if start != wantStart || end != wantEnd {
		t.Errorf("Window(daily) = [%d, %d), want [%d, %d)", start, end, wantStart, wantEnd)
	}
	if end-start != 86400 {
		t.Errorf("day span = %ds, want exactly 86400", end-start)
	}
}

func TestWindow_Week_WednesdayAnchor(t *testing.T) {
	now := time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC) // Wednesday
	start, end := Window("weekly", now)
	wantStart := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC).Unix() // Monday
	wantEnd := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC).Unix()   // next Monday
	if start != wantStart || end != wantEnd {
		t.Errorf("Window(weekly) = [%d, %d), want Mon Aug 17 - Mon Aug 24 [%d, %d)", start, end, wantStart, wantEnd)
	}
}

func TestWindow_Week_SundayAnchor(t *testing.T) {
	// Sunday is the trickiest edge case for a Monday-first week - Go's
	// time.Weekday() returns 0 for Sunday, which must map to "end of
	// THIS week", not "start of a new week".
	now := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC) // Sunday
	start, end := Window("weekly", now)
	wantStart := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC).Unix() // still Monday Aug 17
	wantEnd := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC).Unix()
	if start != wantStart || end != wantEnd {
		t.Errorf("Window(weekly) on Sunday anchor = [%d, %d), want [%d, %d)", start, end, wantStart, wantEnd)
	}
}

func TestWindow_Month(t *testing.T) {
	now := time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC)
	start, end := Window("monthly", now)
	wantStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Unix()
	wantEnd := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Unix()
	if start != wantStart || end != wantEnd {
		t.Errorf("Window(monthly) = [%d, %d), want [%d, %d)", start, end, wantStart, wantEnd)
	}
}

func TestWindow_UnrecognizedSchedulesFallBackToDaily(t *testing.T) {
	now := time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC)
	wantStart, wantEnd := Window("daily", now)
	for _, schedule := range []string{"custom", "", "fortnightly", "DAILY"} {
		start, end := Window(schedule, now)
		if start != wantStart || end != wantEnd {
			t.Errorf("Window(%q) = [%d, %d), want the daily fallback [%d, %d)", schedule, start, end, wantStart, wantEnd)
		}
	}
}
