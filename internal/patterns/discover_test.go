package patterns

import (
	"strings"
	"testing"
	"time"
)

var utc = time.UTC

func unixAt(day int, hour, minute int) int64 {
	return time.Date(2026, time.August, day, hour, minute, 0, 0, utc).Unix()
}

func TestTypicalStartTimes_NotEnoughData(t *testing.T) {
	samples := []SessionSample{
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(1, 10, 30), EndedAt: unixAt(1, 11, 0)},
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(2, 10, 30), EndedAt: unixAt(2, 11, 0)},
	}
	got := TypicalStartTimes(samples, utc)
	if len(got) != 0 {
		t.Fatalf("expected no insights below MinSamplesForTypicalStart, got %+v", got)
	}
}

func TestTypicalStartTimes_ReportsMedian(t *testing.T) {
	// Five sessions clustered around 10:20-10:40, plus one outlier at
	// 03:00 - the median should stay inside the cluster, not get
	// dragged toward the outlier the way a mean would be (a mean of
	// these six would land around 07:54, well outside the cluster).
	samples := []SessionSample{
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(1, 10, 20)},
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(2, 10, 25)},
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(3, 10, 30)},
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(4, 10, 35)},
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(5, 10, 40)},
		{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(6, 3, 0)}, // outlier
	}
	got := TypicalStartTimes(samples, utc)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 insight, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Text, "10:2") && !strings.Contains(got[0].Text, "10:3") {
		t.Errorf("expected a time inside the 10:20-10:40 cluster, got %q", got[0].Text)
	}
	if !strings.HasPrefix(got[0].Text, "STELLAREI observed") {
		t.Errorf("expected the observational phrasing prefix, got %q", got[0].Text)
	}
	if got[0].CategoryID != 1 || got[0].CategoryName != "Development" {
		t.Errorf("unexpected category fields: %+v", got[0])
	}
}

func TestTypicalStartTimes_SkipsUncategorized(t *testing.T) {
	samples := make([]SessionSample, 0, MinSamplesForTypicalStart)
	for i := 1; i <= MinSamplesForTypicalStart; i++ {
		samples = append(samples, SessionSample{CategoryID: 0, StartedAt: unixAt(i, 10, 0)})
	}
	if got := TypicalStartTimes(samples, utc); len(got) != 0 {
		t.Fatalf("expected uncategorized (id 0) sessions never to produce an insight, got %+v", got)
	}
}

func TestTypicalStartTimes_MultipleCategoriesStableOrder(t *testing.T) {
	var samples []SessionSample
	for i := 1; i <= MinSamplesForTypicalStart; i++ {
		samples = append(samples, SessionSample{CategoryID: 2, CategoryName: "Music", StartedAt: unixAt(i, 14, 0)})
		samples = append(samples, SessionSample{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(i, 10, 0)})
	}
	got := TypicalStartTimes(samples, utc)
	if len(got) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(got))
	}
	// Lower category ID first - a stable, deterministic order.
	if got[0].CategoryID != 1 || got[1].CategoryID != 2 {
		t.Errorf("expected stable ascending-category-id order, got ids %d, %d", got[0].CategoryID, got[1].CategoryID)
	}
}

func TestTimeOfDayLengthPatterns_NotEnoughSamplesOnOneSide(t *testing.T) {
	var samples []SessionSample
	for i := 1; i <= MinSamplesForTimeOfDayPattern; i++ {
		// All sessions before the threshold - no "after" samples at all.
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 10, 0), EndedAt: unixAt(i, 10, 0) + 1800,
		})
	}
	if got := TimeOfDayLengthPatterns(samples, utc); len(got) != 0 {
		t.Fatalf("expected no insight without samples on both sides of the threshold, got %+v", got)
	}
}

func TestTimeOfDayLengthPatterns_ReportsLongerAfterEvening(t *testing.T) {
	var samples []SessionSample
	for i := 1; i <= MinSamplesForTimeOfDayPattern; i++ {
		// Before 20:00: 30-minute sessions.
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 10, 0), EndedAt: unixAt(i, 10, 0) + 30*60,
		})
		// At/after 20:00: 90-minute sessions - 3x longer, well above the
		// MinPercentDiffForTimeOfDayPattern threshold.
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 21, 0), EndedAt: unixAt(i, 21, 0) + 90*60,
		})
	}
	got := TimeOfDayLengthPatterns(samples, utc)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 insight, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Text, "20:00") || !strings.Contains(got[0].Text, "longer") {
		t.Errorf("expected the evening-longer phrasing, got %q", got[0].Text)
	}
	if strings.Contains(got[0].Text, "causes") || strings.Contains(got[0].Text, "because") {
		t.Errorf("must never claim causation, got %q", got[0].Text)
	}
}

func TestTimeOfDayLengthPatterns_ReportsLongerBeforeEvening(t *testing.T) {
	var samples []SessionSample
	for i := 1; i <= MinSamplesForTimeOfDayPattern; i++ {
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 10, 0), EndedAt: unixAt(i, 10, 0) + 90*60,
		})
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 21, 0), EndedAt: unixAt(i, 21, 0) + 30*60,
		})
	}
	got := TimeOfDayLengthPatterns(samples, utc)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 insight, got %d", len(got))
	}
	if !strings.Contains(got[0].Text, "before") {
		t.Errorf("expected the before-longer phrasing, got %q", got[0].Text)
	}
}

func TestTimeOfDayLengthPatterns_SmallDifferenceNotReported(t *testing.T) {
	var samples []SessionSample
	for i := 1; i <= MinSamplesForTimeOfDayPattern; i++ {
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 10, 0), EndedAt: unixAt(i, 10, 0) + 60*60,
		})
		// Only 5% longer - below MinPercentDiffForTimeOfDayPattern (20%).
		samples = append(samples, SessionSample{
			CategoryID: 1, CategoryName: "Design",
			StartedAt: unixAt(i, 21, 0), EndedAt: unixAt(i, 21, 0) + 63*60,
		})
	}
	if got := TimeOfDayLengthPatterns(samples, utc); len(got) != 0 {
		t.Fatalf("expected no insight for a difference below the noise threshold, got %+v", got)
	}
}

func TestContextSwitchTrend_NotEnoughSwitches(t *testing.T) {
	_, ok := ContextSwitchTrend(600, 300, MinSwitchesForTrend-1)
	if ok {
		t.Fatal("expected no trend below MinSwitchesForTrend")
	}
}

func TestContextSwitchTrend_NoPriorWeekData(t *testing.T) {
	_, ok := ContextSwitchTrend(600, 0, 10)
	if ok {
		t.Fatal("expected no trend when last week has zero baseline (nothing to compare against)")
	}
}

func TestContextSwitchTrend_SmallChangeNotReported(t *testing.T) {
	// 5% increase - below MinPercentDiffForTrend (15%).
	_, ok := ContextSwitchTrend(315, 300, 10)
	if ok {
		t.Fatal("expected no trend for a change below the noise threshold")
	}
}

func TestContextSwitchTrend_Increased(t *testing.T) {
	insight, ok := ContextSwitchTrend(600, 300, 10) // +100%
	if !ok {
		t.Fatal("expected a trend insight")
	}
	if !strings.Contains(insight.Text, "increased") {
		t.Errorf("expected 'increased', got %q", insight.Text)
	}
	if insight.Kind != "context_switch_trend" {
		t.Errorf("unexpected Kind %q", insight.Kind)
	}
}

func TestContextSwitchTrend_Decreased(t *testing.T) {
	insight, ok := ContextSwitchTrend(150, 300, 10) // -50%
	if !ok {
		t.Fatal("expected a trend insight")
	}
	if !strings.Contains(insight.Text, "decreased") {
		t.Errorf("expected 'decreased', got %q", insight.Text)
	}
}

func TestDiscoverAll_OrderAndComposition(t *testing.T) {
	var samples []SessionSample
	for i := 1; i <= MinSamplesForTypicalStart; i++ {
		samples = append(samples, SessionSample{CategoryID: 1, CategoryName: "Development", StartedAt: unixAt(i, 10, 30)})
	}
	insights := DiscoverAll(samples, utc, 600, 300, 10)
	if len(insights) != 2 {
		t.Fatalf("expected 2 insights (typical-start + context-switch trend), got %d: %+v", len(insights), insights)
	}
	if insights[0].Kind != "typical_start" {
		t.Errorf("expected typical_start first, got %q", insights[0].Kind)
	}
	if insights[len(insights)-1].Kind != "context_switch_trend" {
		t.Errorf("expected context_switch_trend last, got %q", insights[len(insights)-1].Kind)
	}
}

func TestDiscoverAll_EmptyWhenNoDataQualifies(t *testing.T) {
	insights := DiscoverAll(nil, utc, 0, 0, 0)
	if len(insights) != 0 {
		t.Fatalf("expected zero insights for empty input, got %+v", insights)
	}
}

func TestMedianInt(t *testing.T) {
	cases := []struct {
		name   string
		values []int
		want   int
	}{
		{"odd", []int{5, 1, 3}, 3},
		{"even", []int{10, 20, 30, 40}, 25},
		{"single", []int{7}, 7},
		{"unsorted with duplicates", []int{4, 4, 1, 9, 4}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := make([]int, len(tc.values))
			copy(original, tc.values)
			got := medianInt(tc.values)
			if got != tc.want {
				t.Errorf("medianInt(%v) = %d, want %d", tc.values, got, tc.want)
			}
			for i := range tc.values {
				if tc.values[i] != original[i] {
					t.Errorf("medianInt mutated its input slice: got %v, want %v", tc.values, original)
					break
				}
			}
		})
	}
}

func TestClockLabel(t *testing.T) {
	cases := map[int]string{
		0:    "00:00",
		630:  "10:30",
		1439: "23:59",
		61:   "01:01",
	}
	for minute, want := range cases {
		if got := clockLabel(minute); got != want {
			t.Errorf("clockLabel(%d) = %q, want %q", minute, got, want)
		}
	}
}

func TestDurationSeconds_ClampsNegative(t *testing.T) {
	s := SessionSample{StartedAt: 1000, EndedAt: 500} // inverted, shouldn't happen upstream
	if got := s.DurationSeconds(); got != 0 {
		t.Errorf("DurationSeconds() = %d, want 0 for an inverted sample", got)
	}
}
