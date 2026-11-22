package patterns

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

// generateSyntheticSessions produces a plausible-looking, fully
// deterministic (seeded) multi-year stream of session samples across a
// handful of categories, for the long-run benchmark below - spec
// section 79's "Long-Run Test": "Create a synthetic activity generator
// ... Generate 1 day / 1 week / 1 month / 1 year of activity data. Use
// it to benchmark ... analytics ... The application must remain
// responsive."
//
// Scope note, stated honestly rather than implied: this generator only
// produces the shape internal/patterns consumes (SessionSample - start/
// end/category), and this file only benchmarks pattern-detection
// itself. Section 52's OTHER long-run asks - database size, real SQLite
// query performance, and the Timeline's rendering cost - are NOT
// exercised here, because this sandbox has no real `modernc.org/sqlite`
// driver to write that much data through (see README's "What's
// verified vs. what isn't"). This benchmark verifies the one piece of
// "must remain responsive over a year of data" that pure Go, driver-free
// computation actually lets us check for real.
func generateSyntheticSessions(years int, seed int64) []SessionSample {
	rng := rand.New(rand.NewSource(seed))
	categories := []struct {
		id   int64
		name string
	}{
		{1, "Development"}, {2, "Design"}, {3, "Music"},
		{4, "Gaming"}, {5, "Research"}, {6, "Communication"},
	}

	start := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	totalDays := years * 365
	samples := make([]SessionSample, 0, totalDays*4)

	for day := 0; day < totalDays; day++ {
		dayStart := start.AddDate(0, 0, day)
		sessionsToday := 2 + rng.Intn(5)      // 2-6 sessions/day, a realistic daily spread
		cursorMinutes := 8*60 + rng.Intn(120) // usually starts sometime in the morning
		for i := 0; i < sessionsToday; i++ {
			cat := categories[rng.Intn(len(categories))]
			durationMinutes := 10 + rng.Intn(110) // 10-120 minute sessions
			startedAt := dayStart.Add(time.Duration(cursorMinutes) * time.Minute)
			endedAt := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
			samples = append(samples, SessionSample{
				CategoryID:   cat.id,
				CategoryName: cat.name,
				StartedAt:    startedAt.Unix(),
				EndedAt:      endedAt.Unix(),
			})
			// Small gap before the next session, plus occasional bigger
			// breaks - keeps sessions from overlapping while staying
			// unpredictable enough not to look hand-tuned.
			cursorMinutes += durationMinutes + 5 + rng.Intn(90)
			if cursorMinutes > 23*60 {
				break // ran out of day - realistic, not every day fits 6 sessions
			}
		}
	}
	return samples
}

// TestGenerateSyntheticSessions_IsDeterministic pins the one property
// every other benchmark/test in this file depends on: the same seed
// must always produce the same data, or a benchmark "getting faster" or
// "getting slower" between runs would be meaningless noise instead of a
// real signal.
func TestGenerateSyntheticSessions_IsDeterministic(t *testing.T) {
	a := generateSyntheticSessions(1, 42)
	b := generateSyntheticSessions(1, 42)
	if len(a) != len(b) {
		t.Fatalf("same seed produced different sample counts: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed produced different sample at index %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestDiscoverAll_OneYearOfData is a regular (non-benchmark) correctness
// check over a full year of synthetic data: every category above the
// sample-size thresholds should produce a typical-start insight, none of
// the produced text should ever contain causal language, and - the part
// a benchmark alone can't confirm - the RESULTS have to still make
// sense at this volume, not just run fast.
func TestDiscoverAll_OneYearOfData(t *testing.T) {
	samples := generateSyntheticSessions(1, 42)
	if len(samples) < 365*2 {
		t.Fatalf("generator produced too little data to be a meaningful year-long test: %d samples", len(samples))
	}

	insights := DiscoverAll(samples, time.UTC, 600, 300, 10)
	if len(insights) == 0 {
		t.Fatal("expected at least one insight from a full year of 6-category data")
	}
	for _, in := range insights {
		for _, forbidden := range []string{"causes", "because", "leads to"} {
			if strings.Contains(in.Text, forbidden) {
				t.Errorf("insight text must never claim causation, found %q in: %s", forbidden, in.Text)
			}
		}
	}

	// Every one of the 6 synthetic categories has far more than
	// MinSamplesForTypicalStart sessions/year, so every one of them
	// should produce a typical-start insight - a coarse sanity check
	// that the generator's category spread and the detector's threshold
	// actually interact the way both were designed to.
	typicalStartCount := 0
	for _, in := range insights {
		if in.Kind == "typical_start" {
			typicalStartCount++
		}
	}
	if typicalStartCount != 6 {
		t.Errorf("expected a typical-start insight for all 6 synthetic categories, got %d", typicalStartCount)
	}
}

// BenchmarkDiscoverAll_OneYear measures Pattern Discovery over roughly a
// year of realistic multi-category session data - spec section 79's
// "must remain responsive" for this subsystem specifically. Run with:
//
//	go test -bench BenchmarkDiscoverAll -benchtime=10x ./internal/patterns/...
func BenchmarkDiscoverAll_OneYear(b *testing.B) {
	samples := generateSyntheticSessions(1, 42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverAll(samples, time.UTC, 600, 300, 10)
	}
}

// BenchmarkDiscoverAll_ThreeYears is the same check at 3x the volume
// years of continuous background tracking is exactly the "months or
// years of continuous operation" scenario spec section 1 says STELLAREI
// must hold up under, so this benchmark exists specifically to catch an
// accidentally-quadratic implementation before it ships, not just to
// report a number nobody looks at again.
func BenchmarkDiscoverAll_ThreeYears(b *testing.B) {
	samples := generateSyntheticSessions(3, 42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverAll(samples, time.UTC, 600, 300, 10)
	}
}
