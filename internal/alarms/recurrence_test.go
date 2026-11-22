package alarms

import (
	"reflect"
	"testing"
	"time"
)

func TestParseRecurrenceRule_Daily(t *testing.T) {
	r, err := ParseRecurrenceRule("FREQ=DAILY;TIME=09:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Hour != 9 || r.Minute != 0 {
		t.Fatalf("got %02d:%02d, want 09:00", r.Hour, r.Minute)
	}
	if len(r.Weekdays) != 0 {
		t.Fatalf("DAILY should have no weekday restriction, got %v", r.Weekdays)
	}
}

func TestParseRecurrenceRule_Weekdays(t *testing.T) {
	r, err := ParseRecurrenceRule("FREQ=WEEKDAYS;TIME=09:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	if !reflect.DeepEqual(sortedWeekdays(r.Weekdays), sortedWeekdays(want)) {
		t.Fatalf("got %v, want %v", r.Weekdays, want)
	}
}

func TestParseRecurrenceRule_WeeklyByDay(t *testing.T) {
	r, err := ParseRecurrenceRule("FREQ=WEEKLY;BYDAY=MO,WE,FR;TIME=18:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Hour != 18 || r.Minute != 30 {
		t.Fatalf("got %02d:%02d, want 18:30", r.Hour, r.Minute)
	}
	want := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	if !reflect.DeepEqual(sortedWeekdays(r.Weekdays), sortedWeekdays(want)) {
		t.Fatalf("got %v, want %v", r.Weekdays, want)
	}
}

func TestParseRecurrenceRule_MissingTime(t *testing.T) {
	if _, err := ParseRecurrenceRule("FREQ=DAILY"); err == nil {
		t.Fatal("expected an error for a rule with no TIME")
	}
}

func TestParseRecurrenceRule_BadHour(t *testing.T) {
	if _, err := ParseRecurrenceRule("FREQ=DAILY;TIME=25:00"); err == nil {
		t.Fatal("expected an error for an out-of-range hour")
	}
}

func TestParseRecurrenceRule_UnknownFreq(t *testing.T) {
	if _, err := ParseRecurrenceRule("FREQ=YEARLY;TIME=09:00"); err == nil {
		t.Fatal("expected an error for an unsupported FREQ")
	}
}

func TestParseRecurrenceRule_UnknownByDay(t *testing.T) {
	if _, err := ParseRecurrenceRule("FREQ=WEEKLY;BYDAY=XX;TIME=09:00"); err == nil {
		t.Fatal("expected an error for an unrecognized BYDAY token")
	}
}

func TestNextFireAfter_DailyLaterToday(t *testing.T) {
	r, _ := ParseRecurrenceRule("FREQ=DAILY;TIME=18:00")
	now := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC) // a Sunday
	got := r.NextFireAfter(now)
	want := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNextFireAfter_DailyAlreadyPassedTodayRollsToTomorrow(t *testing.T) {
	r, _ := ParseRecurrenceRule("FREQ=DAILY;TIME=09:00")
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) // past 9am already
	got := r.NextFireAfter(now)
	want := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNextFireAfter_ExactlyAtFireTimeRollsToNextDay(t *testing.T) {
	// "after" the exact fire instant means the NEXT occurrence, not the
	// same one again - NextFireAfter must return something strictly
	// after `after`, otherwise the scheduler could re-fire the same
	// instant forever.
	r, _ := ParseRecurrenceRule("FREQ=DAILY;TIME=09:00")
	now := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	got := r.NextFireAfter(now)
	want := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNextFireAfter_WeekdaysSkipsWeekend(t *testing.T) {
	r, _ := ParseRecurrenceRule("FREQ=WEEKDAYS;TIME=09:00")
	// Friday 2026-08-14, after 9am - next weekday occurrence should be
	// Monday 2026-08-17, skipping Sat 15th and Sun 16th.
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	got := r.NextFireAfter(now)
	want := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v (%s), want %v (%s)", got, got.Weekday(), want, want.Weekday())
	}
}

func TestNextFireAfter_WeeklySpecificDays(t *testing.T) {
	r, _ := ParseRecurrenceRule("FREQ=WEEKLY;BYDAY=MO,FR;TIME=07:00")
	// Monday 2026-08-17, after 7am fire - next should be Friday the 21st.
	now := time.Date(2026, 8, 17, 8, 0, 0, 0, time.UTC)
	got := r.NextFireAfter(now)
	want := time.Date(2026, 8, 21, 7, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v (%s), want %v (%s)", got, got.Weekday(), want, want.Weekday())
	}
}

func TestNextFireAfter_PreservesLocation(t *testing.T) {
	loc := time.FixedZone("UTC+3:30", 3*3600+30*60) // Iran Standard Time offset
	r, _ := ParseRecurrenceRule("FREQ=DAILY;TIME=09:00")
	now := time.Date(2026, 8, 16, 6, 0, 0, 0, loc)
	got := r.NextFireAfter(now)
	if got.Location() != loc {
		t.Fatalf("location = %v, want %v", got.Location(), loc)
	}
	if got.Hour() != 9 || got.Minute() != 0 {
		t.Fatalf("local time = %02d:%02d, want 09:00", got.Hour(), got.Minute())
	}
}
