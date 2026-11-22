package waste

import (
	"strings"
	"testing"
)

func session(catID int64, catName string, startSec, durationSec int64) SessionSample {
	return SessionSample{CategoryID: catID, CategoryName: catName, StartedAt: startSec, EndedAt: startSec + durationSec}
}

func TestRepeatedReopens_NotEnoughSessions(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < MinSessionsForRepeatedReopens-1; i++ {
		samples = append(samples, session(1, "Communication", int64(i*100), 60))
	}
	if got := RepeatedReopens(samples); len(got) != 0 {
		t.Fatalf("expected no findings below the session-count threshold, got %+v", got)
	}
}

func TestRepeatedReopens_SessionsTooLong(t *testing.T) {
	// Enough sessions, but each averages well over
	// MaxAvgMinutesForRepeatedReopens - ordinary sustained work, not a
	// reopening pattern.
	var samples []SessionSample
	for i := 0; i < MinSessionsForRepeatedReopens+5; i++ {
		samples = append(samples, session(1, "Development", int64(i*10000), 45*60))
	}
	if got := RepeatedReopens(samples); len(got) != 0 {
		t.Fatalf("expected no findings when sessions are long (ordinary use), got %+v", got)
	}
}

func TestRepeatedReopens_ReportsShortFrequentSessions(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 30; i++ {
		samples = append(samples, session(2, "Communication", int64(i*1000), 90)) // 1.5 min each
	}
	got := RepeatedReopens(samples)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Text, "30 times") {
		t.Errorf("expected the count in the text, got %q", got[0].Text)
	}
	if !strings.HasPrefix(got[0].Text, "STELLAREI observed") {
		t.Errorf("expected observational phrasing, got %q", got[0].Text)
	}
	for _, judgmental := range []string{"waste", "bad", "should", "too much", "stop"} {
		if strings.Contains(strings.ToLower(got[0].Text), judgmental) {
			t.Errorf("must never make a moral judgment, found %q in: %s", judgmental, got[0].Text)
		}
	}
}

func TestRepeatedReopens_SkipsUncategorized(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 30; i++ {
		samples = append(samples, session(0, "", int64(i*1000), 60))
	}
	if got := RepeatedReopens(samples); len(got) != 0 {
		t.Fatalf("expected uncategorized (id 0) sessions never to produce a finding, got %+v", got)
	}
}

func TestLongIdlePeriods_NotEnough(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < MinCountForLongIdlePeriods-1; i++ {
		samples = append(samples, session(9, "Idle", int64(i*10000), (LongIdleThresholdMinutes+5)*60))
	}
	_, ok := LongIdlePeriods(samples, 9)
	if ok {
		t.Fatal("expected no finding below MinCountForLongIdlePeriods")
	}
}

func TestLongIdlePeriods_TooShortToQualify(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 10; i++ {
		samples = append(samples, session(9, "Idle", int64(i*10000), 5*60)) // only 5 min, below threshold
	}
	_, ok := LongIdlePeriods(samples, 9)
	if ok {
		t.Fatal("expected no finding when idle sessions are shorter than LongIdleThresholdMinutes")
	}
}

func TestLongIdlePeriods_Reports(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 4; i++ {
		samples = append(samples, session(9, "Idle", int64(i*10000), 40*60)) // 40 min each, x4 = 160 min = 2h40m
	}
	// Non-idle sessions must not be counted.
	samples = append(samples, session(1, "Development", 999999, 3*3600))

	f, ok := LongIdlePeriods(samples, 9)
	if !ok {
		t.Fatal("expected a finding")
	}
	if !strings.Contains(f.Text, "4 idle periods") {
		t.Errorf("expected the count, got %q", f.Text)
	}
	if !strings.Contains(f.Text, "2h 40m") {
		t.Errorf("expected the total duration formatted as 2h 40m, got %q", f.Text)
	}
}

func TestLongIdlePeriods_ZeroCategoryID(t *testing.T) {
	// idleCategoryID == 0 means "the caller couldn't resolve an Idle
	// category" - must degrade to no finding, never accidentally match
	// samples whose own CategoryID also happens to be the zero value
	// (uncategorized), which would otherwise misreport uncategorized
	// time as "idle".
	samples := []SessionSample{session(0, "", 0, 9999*60)}
	_, ok := LongIdlePeriods(samples, 0)
	if ok {
		t.Fatal("expected no finding when idleCategoryID is unresolved (0)")
	}
}

func TestShortNoiseSessions_NotEnough(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < MinCountForShortNoiseSessions-1; i++ {
		samples = append(samples, session(1, "Development", int64(i*1000), 30))
	}
	_, ok := ShortNoiseSessions(samples, 9)
	if ok {
		t.Fatal("expected no finding below MinCountForShortNoiseSessions")
	}
}

func TestShortNoiseSessions_ExcludesIdle(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 20; i++ {
		samples = append(samples, session(9, "Idle", int64(i*1000), 30)) // all idle, all short
	}
	_, ok := ShortNoiseSessions(samples, 9)
	if ok {
		t.Fatal("expected idle sessions to be excluded from short-noise detection")
	}
}

func TestShortNoiseSessions_ExcludesZeroDuration(t *testing.T) {
	// A zero-duration row (e.g. a data glitch) must not count as
	// "noise" - it's not evidence of anything, just an edge case that
	// shouldn't inflate the finding.
	var samples []SessionSample
	for i := 0; i < 20; i++ {
		samples = append(samples, session(1, "Development", int64(i*1000), 0))
	}
	_, ok := ShortNoiseSessions(samples, 9)
	if ok {
		t.Fatal("expected zero-duration sessions not to count as noise")
	}
}

func TestShortNoiseSessions_Reports(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 20; i++ {
		samples = append(samples, session(1, "Development", int64(i*1000), 45))
	}
	f, ok := ShortNoiseSessions(samples, 9)
	if !ok {
		t.Fatal("expected a finding")
	}
	if !strings.Contains(f.Text, "20 sessions") {
		t.Errorf("expected the count, got %q", f.Text)
	}
}

func TestDetectAll_OrderAndComposition(t *testing.T) {
	var samples []SessionSample
	for i := 0; i < 30; i++ {
		samples = append(samples, session(2, "Communication", int64(i*1000), 90))
	}
	for i := 0; i < 4; i++ {
		samples = append(samples, session(9, "Idle", int64(1_000_000+i*10000), 40*60))
	}
	// Deliberately NOT short enough to also trigger RepeatedReopens
	// (average must stay under MaxAvgMinutesForRepeatedReopens=5min)
	// these 20 sessions average 45 SECONDS, which is short enough to
	// legitimately qualify for BOTH repeated_reopens AND
	// short_noise_sessions if left as-is (correct behavior, since a
	// category opened 20 times for under a minute each really is both
	// patterns at once) - so this test uses a category with only 5
	// sessions (below MinSessionsForRepeatedReopens=20) specifically to
	// isolate short_noise_sessions from repeated_reopens and keep this
	// test's "exactly 3, in this order" assertion meaningful.
	for i := 0; i < 5; i++ {
		samples = append(samples, session(1, "Development", int64(2_000_000+i*1000), 45))
	}
	// ...plus enough OTHER short sessions across a category that never
	// reaches RepeatedReopens' count threshold, so ShortNoiseSessions
	// still has enough total evidence (it doesn't group by category).
	for i := 0; i < 15; i++ {
		samples = append(samples, session(3, "Research", int64(3_000_000+i*1000), 45))
	}

	findings := DetectAll(samples, 9)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %+v", len(findings), findings)
	}
	if findings[0].Kind != "repeated_reopens" {
		t.Errorf("expected repeated_reopens first, got %q", findings[0].Kind)
	}
	if findings[1].Kind != "long_idle_periods" {
		t.Errorf("expected long_idle_periods second, got %q", findings[1].Kind)
	}
	if findings[2].Kind != "short_noise_sessions" {
		t.Errorf("expected short_noise_sessions third, got %q", findings[2].Kind)
	}
}

func TestDetectAll_EmptyWhenNoDataQualifies(t *testing.T) {
	if got := DetectAll(nil, 9); len(got) != 0 {
		t.Fatalf("expected zero findings for empty input, got %+v", got)
	}
}

func TestFormatHoursMinutes(t *testing.T) {
	cases := map[int64]string{
		0:         "0m",
		59:        "0m",
		60:        "1m",
		3599:      "59m",
		3600:      "1h 0m",
		9600:      "2h 40m",
		25 * 3600: "25h 0m",
	}
	for seconds, want := range cases {
		if got := formatHoursMinutes(seconds); got != want {
			t.Errorf("formatHoursMinutes(%d) = %q, want %q", seconds, got, want)
		}
	}
}
