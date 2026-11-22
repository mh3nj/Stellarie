// Package periods computes the current [start, end) window for a
// schedule string ("daily"/"day", "weekly"/"week", "monthly"/"month"),
// shared by internal/budgets and internal/goals so both compute "this
// week" identically instead of each having their own subtly different
// definition. Mirrors frontend/src/lib/time-ranges.ts's rangeFor
// same Monday-week-start convention - so a budget's "actual so far"
// number matches what Timeline shows for the same stretch of time.
package periods

import "time"

// Window returns [startUnix, endUnix) for the period containing `now`,
// in now's own location (so a person's "today" is their local calendar
// day, not a UTC one - section 70). Unrecognized schedules (including
// "custom", which needs explicit stored bounds no budget/goal schema
// field currently carries) fall back to "daily" rather than erroring
// a budget or goal must always have SOME window to report progress
// against.
func Window(schedule string, now time.Time) (startUnix, endUnix int64) {
	switch schedule {
	case "weekly", "week":
		return weekWindow(now)
	case "monthly", "month":
		return monthWindow(now)
	default: // "daily", "day", "custom", anything unrecognized
		return dayWindow(now)
	}
}

func dayWindow(now time.Time) (int64, int64) {
	y, m, d := now.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	return start.Unix(), end.Unix()
}

func weekWindow(now time.Time) (int64, int64) {
	y, m, d := now.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	// Monday-first week, matching the frontend's rangeFor.
	weekday := int(start.Weekday()) // 0 = Sunday
	daysSinceMonday := (weekday + 6) % 7
	start = start.AddDate(0, 0, -daysSinceMonday)
	end := start.AddDate(0, 0, 7)
	return start.Unix(), end.Unix()
}

func monthWindow(now time.Time) (int64, int64) {
	y, m, _ := now.Date()
	start := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	return start.Unix(), end.Unix()
}
