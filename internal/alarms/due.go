package alarms

import "time"

// Due reports whether a (non-activity-based - see condition.go /
// internal/scheduler for that case, which additionally needs live
// session data this package doesn't have) alarm should fire as of `now`,
// and if so, the fire time to record as LastFiredAt.
//
// Pure function of the alarm's own fields plus `now` - internal/scheduler
// calls this once per tick per alarm; this package's tests call it
// directly with fabricated alarms and times, the same pattern
// recurrence_test.go uses for NextFireAfter.
//
// Rules, checked in order:
//  1. Disabled alarms are never due.
//  2. A snooze in effect (SnoozedUntil set) overrides everything else
//     once now reaches it, the alarm is due regardless of its normal
//     schedule, and firing clears the snooze (the caller is responsible
//     for that - see internal/scheduler).
//  3. A recurring rule (RecurrenceRule set) is due once `now` has passed
//     the next occurrence after LastFiredAt (or after CreatedAt if it has
//     never fired) - this is what makes a recurring alarm keep firing on
//     schedule indefinitely rather than only once.
//  4. A plain FireAt (one_time/relative/schedule with no recurrence) is
//     due once, the first time now reaches it - LastFiredAt already being
//     set means it already fired and must never fire again.
func Due(a Alarm, now time.Time) (due bool, fireAt time.Time) {
	if !a.Enabled {
		return false, time.Time{}
	}

	if a.SnoozedUntil != nil && !now.Before(*a.SnoozedUntil) {
		return true, now
	}
	// While snoozed but not yet due, nothing else about the alarm's
	// normal schedule matters - the snooze is the only thing being
	// waited on. (A snooze past its normal recurrence's next tick is a
	// deliberate choice the person made by snoozing; it isn't overridden
	// by the alarm separately becoming "due" again underneath it.)
	if a.SnoozedUntil != nil {
		return false, time.Time{}
	}

	if a.RecurrenceRule != "" {
		rule, err := ParseRecurrenceRule(a.RecurrenceRule)
		if err != nil {
			// A malformed rule (shouldn't happen - validated at save
			// time - but defensively) must never crash the scheduler or
			// spam notifications; treat as not-yet-due.
			return false, time.Time{}
		}
		basis := a.CreatedAt
		if a.LastFiredAt != nil {
			basis = *a.LastFiredAt
		}
		next := rule.NextFireAfter(basis)
		if !now.Before(next) {
			return true, now
		}
		return false, time.Time{}
	}

	if a.FireAt == nil {
		return false, time.Time{}
	}
	if a.LastFiredAt != nil {
		// One-shot semantics: a plain FireAt with no recurrence fires
		// exactly once, ever.
		return false, time.Time{}
	}
	if !now.Before(*a.FireAt) {
		return true, now
	}
	return false, time.Time{}
}
