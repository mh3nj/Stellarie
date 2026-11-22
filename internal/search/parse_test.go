package search

import (
	"testing"
	"time"
)

var utc = time.UTC

// "Now" anchor for every relative-phrase test: a Wednesday, so
// "this week"/"last week" have an unambiguous, non-edge-case answer.
var fixedNow = time.Date(2026, time.August, 19, 15, 0, 0, 0, utc) // Wednesday

func TestParse_Today(t *testing.T) {
	q := Parse("what did I do today", nil, fixedNow, utc)
	if !q.HasDateRange {
		t.Fatal("expected a date range")
	}
	wantStart := time.Date(2026, 8, 19, 0, 0, 0, 0, utc).Unix()
	wantEnd := time.Date(2026, 8, 20, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != wantStart || q.RangeEnd != wantEnd {
		t.Errorf("got range [%d,%d), want [%d,%d)", q.RangeStart, q.RangeEnd, wantStart, wantEnd)
	}
}

func TestParse_Yesterday(t *testing.T) {
	q := Parse("yesterday", nil, fixedNow, utc)
	wantStart := time.Date(2026, 8, 18, 0, 0, 0, 0, utc).Unix()
	wantEnd := time.Date(2026, 8, 19, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != wantStart || q.RangeEnd != wantEnd {
		t.Errorf("got range [%d,%d), want [%d,%d)", q.RangeStart, q.RangeEnd, wantStart, wantEnd)
	}
}

func TestParse_ThisWeek(t *testing.T) {
	// fixedNow is Wednesday Aug 19 2026 - the Monday of that week is
	// Aug 17.
	q := Parse("this week", nil, fixedNow, utc)
	wantStart := time.Date(2026, 8, 17, 0, 0, 0, 0, utc).Unix()
	wantEnd := time.Date(2026, 8, 24, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != wantStart || q.RangeEnd != wantEnd {
		t.Errorf("got range [%d,%d), want [%d,%d)", q.RangeStart, q.RangeEnd, wantStart, wantEnd)
	}
}

func TestParse_LastWeek(t *testing.T) {
	q := Parse("last week", nil, fixedNow, utc)
	wantStart := time.Date(2026, 8, 10, 0, 0, 0, 0, utc).Unix()
	wantEnd := time.Date(2026, 8, 17, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != wantStart || q.RangeEnd != wantEnd {
		t.Errorf("got range [%d,%d), want [%d,%d)", q.RangeStart, q.RangeEnd, wantStart, wantEnd)
	}
}

func TestParse_ThisMonthAndLastMonth(t *testing.T) {
	q := Parse("this month", nil, fixedNow, utc)
	if q.RangeStart != time.Date(2026, 8, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("this month start wrong: %d", q.RangeStart)
	}
	if q.RangeEnd != time.Date(2026, 9, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("this month end wrong: %d", q.RangeEnd)
	}

	q2 := Parse("gaming last month", nil, fixedNow, utc)
	if q2.RangeStart != time.Date(2026, 7, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("last month start wrong: %d", q2.RangeStart)
	}
	if q2.RangeEnd != time.Date(2026, 8, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("last month end wrong: %d", q2.RangeEnd)
	}
}

func TestParse_ThisYearAndLastYear(t *testing.T) {
	q := Parse("this year", nil, fixedNow, utc)
	if q.RangeStart != time.Date(2026, 1, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("this year start wrong: %d", q.RangeStart)
	}
	q2 := Parse("last year", nil, fixedNow, utc)
	if q2.RangeStart != time.Date(2025, 1, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("last year start wrong: %d", q2.RangeStart)
	}
	if q2.RangeEnd != time.Date(2026, 1, 1, 0, 0, 0, 0, utc).Unix() {
		t.Errorf("last year end wrong: %d", q2.RangeEnd)
	}
}

func TestParse_ExplicitMonthDay_PastThisYear(t *testing.T) {
	// fixedNow is August 19 - "August 10" already happened this year.
	q := Parse("Serenity August 10", nil, fixedNow, utc)
	if !q.HasDateRange {
		t.Fatal("expected a date range")
	}
	want := time.Date(2026, 8, 10, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != want {
		t.Errorf("got start %d, want %d (Aug 10 this year)", q.RangeStart, want)
	}
}

func TestParse_ExplicitMonthDay_FutureAssumesLastYear(t *testing.T) {
	// fixedNow is August 19 - "December 25" hasn't happened yet this
	// year, so it should resolve to LAST December, not next.
	q := Parse("December 25", nil, fixedNow, utc)
	want := time.Date(2025, 12, 25, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != want {
		t.Errorf("got start %d, want %d (Dec 25 LAST year)", q.RangeStart, want)
	}
}

func TestParse_ExplicitMonthDayYear(t *testing.T) {
	q := Parse("August 10, 2020", nil, fixedNow, utc)
	want := time.Date(2020, 8, 10, 0, 0, 0, 0, utc).Unix()
	if q.RangeStart != want {
		t.Errorf("got start %d, want %d", q.RangeStart, want)
	}
}

func TestParse_TimeOfDayWithExplicitDate_ProducesInstant(t *testing.T) {
	q := Parse("What was I doing at 14:30 on August 3", nil, fixedNow, utc)
	if !q.HasInstant {
		t.Fatal("expected an instant, not a range")
	}
	if q.HasDateRange {
		t.Error("expected HasDateRange to be false when an instant is resolved")
	}
	want := time.Date(2026, 8, 3, 14, 30, 0, 0, utc).Unix()
	if q.InstantUnix != want {
		t.Errorf("got instant %d, want %d", q.InstantUnix, want)
	}
}

func TestParse_TimeOfDayWithToday_ProducesInstant(t *testing.T) {
	q := Parse("what was I doing at 09:15 today", nil, fixedNow, utc)
	if !q.HasInstant {
		t.Fatal("expected an instant")
	}
	want := time.Date(2026, 8, 19, 9, 15, 0, 0, utc).Unix()
	if q.InstantUnix != want {
		t.Errorf("got instant %d, want %d", q.InstantUnix, want)
	}
}

func TestParse_TimeOfDayWithMultiDayRange_StaysARange(t *testing.T) {
	// "14:30" combined with "this week" (a multi-day range) shouldn't
	// collapse into a nonsensical single instant - dayResolved only
	// applies to single-day phrases (today/yesterday/explicit date).
	q := Parse("14:30 this week", nil, fixedNow, utc)
	if q.HasInstant {
		t.Error("expected no instant for a multi-day range combined with a time")
	}
	if !q.HasDateRange {
		t.Error("expected the range to still be recognized")
	}
}

func TestParse_Superlative(t *testing.T) {
	for _, tc := range []struct{ query, want string }{
		{"longest coding session", "longest"},
		{"what was my SHORTEST session", "shortest"},
		{"just a normal query", ""},
	} {
		q := Parse(tc.query, nil, fixedNow, utc)
		if q.Superlative != tc.want {
			t.Errorf("Parse(%q).Superlative = %q, want %q", tc.query, q.Superlative, tc.want)
		}
	}
}

func TestParse_EntityMatch_WholeWordOnly(t *testing.T) {
	entities := []KnownEntity{
		{Kind: EntityProject, ID: 1, Name: "Serenity"},
		{Kind: EntityCategory, ID: 2, Name: "Art"},
	}
	// "August" contains "Art"-like substrings nowhere, but this test
	// specifically checks that a short entity name doesn't accidentally
	// match inside an unrelated longer word - using a name that WOULD
	// be a substring match if boundaries weren't respected.
	q := Parse("How much time did I spend arting around", entities, fixedNow, utc)
	if q.Entity != nil {
		t.Errorf("expected no entity match (whole-word boundary should prevent it), got %+v", q.Entity)
	}
}

func TestParse_EntityMatch_Found(t *testing.T) {
	entities := []KnownEntity{
		{Kind: EntityProject, ID: 1, Name: "HeartLib"},
		{Kind: EntityCategory, ID: 2, Name: "Gaming"},
	}
	q := Parse("When did I work on HeartLib?", entities, fixedNow, utc)
	if q.Entity == nil {
		t.Fatal("expected an entity match")
	}
	if q.Entity.Name != "HeartLib" || q.Entity.Kind != EntityProject {
		t.Errorf("got %+v", q.Entity)
	}
}

func TestParse_EntityMatch_PrefersLongestName(t *testing.T) {
	entities := []KnownEntity{
		{Kind: EntityCategory, ID: 1, Name: "Design"},
		{Kind: EntityProject, ID: 2, Name: "Design System Overhaul"},
	}
	q := Parse("time on Design System Overhaul this week", entities, fixedNow, utc)
	if q.Entity == nil || q.Entity.ID != 2 {
		t.Fatalf("expected the longer, more specific match to win, got %+v", q.Entity)
	}
}

func TestParse_EntityMatch_CaseInsensitive(t *testing.T) {
	entities := []KnownEntity{{Kind: EntityCategory, ID: 1, Name: "Gaming"}}
	q := Parse("GAMING last month", entities, fixedNow, utc)
	if q.Entity == nil {
		t.Fatal("expected a case-insensitive match")
	}
}

func TestParse_EntityMatch_RegexSpecialCharsInName(t *testing.T) {
	// A project literally named with regex-special characters must not
	// break the matcher or panic.
	entities := []KnownEntity{{Kind: EntityProject, ID: 1, Name: "C++ Notes"}}
	q := Parse("worked on C++ Notes today", entities, fixedNow, utc)
	if q.Entity == nil || q.Entity.Name != "C++ Notes" {
		t.Errorf("expected a match on the regex-special-character name, got %+v", q.Entity)
	}
}

func TestParse_Unmatched_WhenNothingRecognized(t *testing.T) {
	entities := []KnownEntity{{Kind: EntityCategory, ID: 1, Name: "Gaming"}}
	q := Parse("random thoughts about nothing in particular", entities, fixedNow, utc)
	if !q.Unmatched {
		t.Error("expected Unmatched=true when neither an entity nor a date was recognized")
	}
}

func TestParse_NotUnmatched_WhenEntityFound(t *testing.T) {
	entities := []KnownEntity{{Kind: EntityCategory, ID: 1, Name: "Gaming"}}
	q := Parse("gaming", entities, fixedNow, utc)
	if q.Unmatched {
		t.Error("expected Unmatched=false when an entity was recognized")
	}
}

func TestParse_NotUnmatched_WhenDateFound(t *testing.T) {
	q := Parse("today", nil, fixedNow, utc)
	if q.Unmatched {
		t.Error("expected Unmatched=false when a date was recognized")
	}
}
