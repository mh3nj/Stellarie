package calendar

import (
	"testing"
	"time"
)

var utc = time.UTC

func unixAt(y int, mo time.Month, d, h, m int) int64 {
	return time.Date(y, mo, d, h, m, 0, 0, utc).Unix()
}

func TestDailyCategoryTotals_SessionWithinOneDay(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(2026, 8, 16, 9, 0), EndedAt: unixAt(2026, 8, 16, 11, 0)},
	}
	got := DailyCategoryTotals(samples, utc)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 day, got %d: %+v", len(got), got)
	}
	day, ok := got["2026-08-16"]
	if !ok {
		t.Fatalf("expected an entry for 2026-08-16, got %+v", got)
	}
	if day[1] != 2*3600 {
		t.Errorf("expected 2h (7200s) for category 1, got %ds", day[1])
	}
}

func TestDailyCategoryTotals_SkipsUncategorized(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 0, StartedAt: unixAt(2026, 8, 16, 9, 0), EndedAt: unixAt(2026, 8, 16, 11, 0)},
	}
	got := DailyCategoryTotals(samples, utc)
	if len(got) != 0 {
		t.Fatalf("expected uncategorized (id 0) sessions to produce nothing, got %+v", got)
	}
}

func TestDailyCategoryTotals_SkipsInvertedSession(t *testing.T) {
	// EndedAt <= StartedAt should never happen upstream, but this
	// package doesn't trust its input blindly (same discipline as
	// internal/patterns.SessionSample.DurationSeconds).
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 16, 11, 0), EndedAt: unixAt(2026, 8, 16, 9, 0)},
	}
	got := DailyCategoryTotals(samples, utc)
	if len(got) != 0 {
		t.Fatalf("expected an inverted session to produce nothing, got %+v", got)
	}
}

func TestDailyCategoryTotals_CrossesMidnight_SplitsCorrectly(t *testing.T) {
	// 23:00 Aug 16 -> 01:30 Aug 17 = 1h on the 16th, 1h30m on the 17th.
	samples := []SessionSample{
		{CategoryID: 1, CategoryName: "Gaming", StartedAt: unixAt(2026, 8, 16, 23, 0), EndedAt: unixAt(2026, 8, 17, 1, 30)},
	}
	got := DailyCategoryTotals(samples, utc)
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 days, got %d: %+v", len(got), got)
	}
	if got["2026-08-16"][1] != 3600 {
		t.Errorf("expected 1h (3600s) on the 16th, got %ds", got["2026-08-16"][1])
	}
	if got["2026-08-17"][1] != 90*60 {
		t.Errorf("expected 1h30m (5400s) on the 17th, got %ds", got["2026-08-17"][1])
	}
	// The two parts must sum to the session's total duration exactly
	// no double-counting, no lost seconds at the boundary.
	total := got["2026-08-16"][1] + got["2026-08-17"][1]
	wantTotal := int64(2*3600 + 30*60)
	if total != wantTotal {
		t.Errorf("split parts sum to %ds, want %ds (the session's real total duration)", total, wantTotal)
	}
}

func TestDailyCategoryTotals_BriefMidnightCrossing(t *testing.T) {
	// The package doc's own example: 23:58 -> 00:15 - a session that
	// doesn't "run overnight" in any meaningful sense but still
	// technically touches two days, and both must get their real share.
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 16, 23, 58), EndedAt: unixAt(2026, 8, 17, 0, 15)},
	}
	got := DailyCategoryTotals(samples, utc)
	if got["2026-08-16"][1] != 2*60 {
		t.Errorf("expected 2 minutes on the 16th, got %ds", got["2026-08-16"][1])
	}
	if got["2026-08-17"][1] != 15*60 {
		t.Errorf("expected 15 minutes on the 17th, got %ds", got["2026-08-17"][1])
	}
}

func TestDailyCategoryTotals_SpansMultipleFullDays(t *testing.T) {
	// A pathological but possible case (e.g. a bug elsewhere, or a
	// manually-edited session per spec section 63) spanning 3 calendar
	// days - must split across all three, not just the first two.
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 16, 12, 0), EndedAt: unixAt(2026, 8, 18, 12, 0)}, // 48h exactly
	}
	got := DailyCategoryTotals(samples, utc)
	if len(got) != 3 {
		t.Fatalf("expected 3 days (16th, 17th, 18th), got %d: %+v", len(got), got)
	}
	if got["2026-08-16"][1] != 12*3600 {
		t.Errorf("expected 12h on the 16th (noon to midnight), got %ds", got["2026-08-16"][1])
	}
	if got["2026-08-17"][1] != 24*3600 {
		t.Errorf("expected the full 24h on the 17th, got %ds", got["2026-08-17"][1])
	}
	if got["2026-08-18"][1] != 12*3600 {
		t.Errorf("expected 12h on the 18th (midnight to noon), got %ds", got["2026-08-18"][1])
	}
}

func TestDailyCategoryTotals_MultipleCategoriesSameDay(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(2026, 8, 16, 9, 0), EndedAt: unixAt(2026, 8, 16, 13, 0)},
		{CategoryID: 2, CategoryName: "Gaming", StartedAt: unixAt(2026, 8, 16, 14, 0), EndedAt: unixAt(2026, 8, 16, 15, 0)},
	}
	got := DailyCategoryTotals(samples, utc)
	day := got["2026-08-16"]
	if day[1] != 4*3600 {
		t.Errorf("expected 4h Development, got %ds", day[1])
	}
	if day[2] != 3600 {
		t.Errorf("expected 1h Gaming, got %ds", day[2])
	}
}

func TestDailyCategoryTotals_AccumulatesAcrossMultipleSessionsSameDayCategory(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 16, 9, 0), EndedAt: unixAt(2026, 8, 16, 10, 0)},
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 16, 11, 0), EndedAt: unixAt(2026, 8, 16, 12, 30)},
	}
	got := DailyCategoryTotals(samples, utc)
	if got["2026-08-16"][1] != 3600+90*60 {
		t.Errorf("expected accumulated 2h30m, got %ds", got["2026-08-16"][1])
	}
}

func TestDailyCategoryTotals_DaysWithNoActivityAreAbsent(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 16, 9, 0), EndedAt: unixAt(2026, 8, 16, 10, 0)},
	}
	got := DailyCategoryTotals(samples, utc)
	if _, ok := got["2026-08-15"]; ok {
		t.Error("expected no entry for a day with no activity")
	}
	if _, ok := got["2026-08-17"]; ok {
		t.Error("expected no entry for a day with no activity")
	}
}

func TestSplitByLocalDay_RespectsLocation(t *testing.T) {
	// A location well east of UTC - a session that's entirely within
	// one UTC day can still cross local midnight, and vice versa. Using
	// a fixed offset (rather than a named zone requiring tzdata) keeps
	// this test independent of the container's timezone database.
	loc := time.FixedZone("UTC+10", 10*3600)
	// 22:00 UTC Aug 16 = 08:00 UTC+10 Aug 17 -> ends 23:00 UTC Aug 16 =
	// 09:00 UTC+10 Aug 17. Entirely within local Aug 17, despite being
	// entirely within UTC Aug 16.
	startUTC := unixAt(2026, 8, 16, 22, 0)
	endUTC := unixAt(2026, 8, 16, 23, 0)
	parts := splitByLocalDay(startUTC, endUTC, loc)
	if len(parts) != 1 {
		t.Fatalf("expected exactly 1 part, got %d: %+v", len(parts), parts)
	}
	if parts[0].dayKey != "2026-08-17" {
		t.Errorf("expected local day 2026-08-17, got %q", parts[0].dayKey)
	}
}
