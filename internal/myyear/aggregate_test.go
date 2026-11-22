package myyear

import (
	"testing"
	"time"

	"stellarei/internal/calendar"
)

var utc = time.UTC

func unixAt(y int, mo time.Month, d, h int) int64 {
	return time.Date(y, mo, d, h, 0, 0, 0, utc).Unix()
}

func TestRankByCategory_SortsDescendingAndSkipsUncategorized(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 3600},
		{CategoryID: 2, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 7200},
		{CategoryID: 0, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 99999},
	}
	got := RankByCategory(samples)
	if len(got) != 2 {
		t.Fatalf("expected 2 ranked categories (uncategorized skipped), got %d: %+v", len(got), got)
	}
	if got[0].ID != 2 || got[0].Seconds != 7200 {
		t.Errorf("expected category 2 first with 7200s, got %+v", got[0])
	}
	if got[1].ID != 1 || got[1].Seconds != 3600 {
		t.Errorf("expected category 1 second with 3600s, got %+v", got[1])
	}
}

func TestRankByCategory_AccumulatesMultipleSessions(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 1800},
		{CategoryID: 1, StartedAt: unixAt(2026, 6, 1, 0), EndedAt: unixAt(2026, 6, 1, 0) + 1800},
	}
	got := RankByCategory(samples)
	if len(got) != 1 || got[0].Seconds != 3600 {
		t.Fatalf("expected accumulated 3600s for category 1, got %+v", got)
	}
}

func TestRankByCategory_TieBreaksByID(t *testing.T) {
	// Equal totals must break ties deterministically (by ID), not by Go's
	// randomized map iteration order - otherwise output would reshuffle
	// between identical calls.
	samples := []SessionSample{
		{CategoryID: 5, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 3600},
		{CategoryID: 2, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 3600},
	}
	got := RankByCategory(samples)
	if got[0].ID != 2 || got[1].ID != 5 {
		t.Errorf("expected tie broken by ascending ID (2 before 5), got %+v", got)
	}
}

func TestRankByProject_SkipsUnassigned(t *testing.T) {
	samples := []SessionSample{
		{ProjectID: 0, StartedAt: unixAt(2026, 1, 1, 0), EndedAt: unixAt(2026, 1, 1, 0) + 3600},
	}
	if got := RankByProject(samples); len(got) != 0 {
		t.Fatalf("expected unassigned (id 0) sessions skipped, got %+v", got)
	}
}

func TestLongestSessions_TopNDescending(t *testing.T) {
	samples := []SessionSample{
		{SessionID: 1, ProjectName: "A", StartedAt: 0, EndedAt: 600},
		{SessionID: 2, ProjectName: "B", StartedAt: 0, EndedAt: 3600},
		{SessionID: 3, ProjectName: "C", StartedAt: 0, EndedAt: 1800},
	}
	got := LongestSessions(samples, 2, utc)
	if len(got) != 2 {
		t.Fatalf("expected top 2, got %d", len(got))
	}
	if got[0].SessionID != 2 || got[1].SessionID != 3 {
		t.Errorf("expected [2,3] (longest first), got session ids %d,%d", got[0].SessionID, got[1].SessionID)
	}
}

func TestLongestSessions_TitleFallback(t *testing.T) {
	samples := []SessionSample{
		{SessionID: 1, ProjectName: "", CategoryName: "Development", StartedAt: 0, EndedAt: 600},
		{SessionID: 2, ProjectName: "", CategoryName: "", StartedAt: 0, EndedAt: 600},
	}
	got := LongestSessions(samples, 10, utc)
	titles := map[int64]string{}
	for _, ls := range got {
		titles[ls.SessionID] = ls.Title
	}
	if titles[1] != "Development" {
		t.Errorf("expected category fallback 'Development', got %q", titles[1])
	}
	if titles[2] != "Uncategorized" {
		t.Errorf("expected final fallback 'Uncategorized', got %q", titles[2])
	}
}

func TestLongestSessions_DoesNotMutateInput(t *testing.T) {
	samples := []SessionSample{
		{SessionID: 1, StartedAt: 0, EndedAt: 600},
		{SessionID: 2, StartedAt: 0, EndedAt: 3600},
	}
	original := make([]SessionSample, len(samples))
	copy(original, samples)
	LongestSessions(samples, 1, utc)
	for i := range samples {
		if samples[i] != original[i] {
			t.Fatalf("LongestSessions mutated its input slice at index %d", i)
		}
	}
}

func TestBusiestMonth_EmptyInput(t *testing.T) {
	label, seconds := BusiestMonth(nil, utc)
	if label != "" || seconds != 0 {
		t.Errorf("expected (\"\", 0) for empty input, got (%q, %d)", label, seconds)
	}
}

func TestBusiestMonth_PicksHighestTotal(t *testing.T) {
	samples := []calendar.SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 3, 15, 10), EndedAt: unixAt(2026, 3, 15, 10) + 3600},   // March: 1h
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 10, 10), EndedAt: unixAt(2026, 8, 10, 10) + 5*3600}, // August: 5h
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 20, 10), EndedAt: unixAt(2026, 8, 20, 10) + 3600},   // August: +1h = 6h
	}
	label, seconds := BusiestMonth(samples, utc)
	if label != "August" {
		t.Errorf("expected August, got %q", label)
	}
	if seconds != 6*3600 {
		t.Errorf("expected 6h (21600s), got %ds", seconds)
	}
}

func TestBusiestMonth_TieBreaksByEarlierMonth(t *testing.T) {
	samples := []calendar.SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 3, 15, 10), EndedAt: unixAt(2026, 3, 15, 10) + 3600},
		{CategoryID: 1, StartedAt: unixAt(2026, 8, 15, 10), EndedAt: unixAt(2026, 8, 15, 10) + 3600},
	}
	label, _ := BusiestMonth(samples, utc)
	if label != "March" {
		t.Errorf("expected March (earlier month wins a tie, deterministic), got %q", label)
	}
}

func TestBusiestMonth_SessionCrossingMidnightAttributesCorrectly(t *testing.T) {
	// A session crossing from the last day of July into August 1st must
	// split correctly between the two months, relying on
	// internal/calendar's own already-tested midnight-crossing logic.
	samples := []calendar.SessionSample{
		{CategoryID: 1, StartedAt: unixAt(2026, 7, 31, 22), EndedAt: unixAt(2026, 7, 31, 22) + 4*3600}, // 22:00 Jul31 -> 02:00 Aug1: 2h/2h split
	}
	dayTotals := calendar.DailyCategoryTotals(samples, utc)
	if dayTotals["2026-07-31"][1] != 2*3600 {
		t.Fatalf("sanity check failed: expected 2h on Jul 31, got %ds", dayTotals["2026-07-31"][1])
	}
	if dayTotals["2026-08-01"][1] != 2*3600 {
		t.Fatalf("sanity check failed: expected 2h on Aug 1, got %ds", dayTotals["2026-08-01"][1])
	}
	label, seconds := BusiestMonth(samples, utc)
	// Both July and August have exactly 2h - a genuine tie, so the
	// earlier month (July) should win deterministically.
	if label != "July" {
		t.Errorf("expected July (earlier month, deterministic tie-break), got %q", label)
	}
	if seconds != 2*3600 {
		t.Errorf("expected 2h for the winning month, got %ds", seconds)
	}
}
