package timers

import "time"

// Elapsed computes how much run time a timer has accumulated as of `now`,
// excluding every paused interval. Pure function of the timer's stored
// fields plus `now` - no I/O, no global clock - so both this package's
// tests and internal/scheduler's tests can drive it with fabricated
// times, the same pattern internal/activity's debouncer uses.
//
// Rules:
//   - A completed/cancelled timer's elapsed time is frozen at EndedAt,
//     regardless of what `now` is - history doesn't keep ticking.
//   - A currently-paused timer (PausedAt set) doesn't accrue elapsed time
//     for however long it's been paused, including the ongoing pause not
//     yet folded into AccumulatedPauseSecs.
//   - Elapsed never goes negative (a defensive clamp - it shouldn't be
//     possible given the invariants above, but a corrupted/edited row
//     must not make the UI show a negative countdown).
func Elapsed(t Timer, now time.Time) time.Duration {
	end := now
	if t.EndedAt != nil {
		end = *t.EndedAt
	}

	raw := end.Sub(t.StartedAt)
	pause := time.Duration(t.AccumulatedPauseSecs) * time.Second

	if t.PausedAt != nil && t.EndedAt == nil {
		// Still mid-pause: fold in the ongoing pause, capped at `end` so a
		// PausedAt that's (incorrectly) after `now` can't produce a
		// negative addition.
		if end.After(*t.PausedAt) {
			pause += end.Sub(*t.PausedAt)
		}
	}

	elapsed := raw - pause
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// Remaining computes what's left of a countdown-style timer's target.
// complete reports whether the target has been reached or passed
// internal/scheduler uses this (not a separate threshold check) so the
// "is this timer done" decision has exactly one implementation.
// Stopwatch timers (DurationSecs == 0) are never complete; Remaining is 0
// for them since "remaining" isn't a meaningful concept without a target.
func Remaining(t Timer, now time.Time) (remaining time.Duration, complete bool) {
	if !t.HasTarget() {
		return 0, false
	}
	target := time.Duration(t.DurationSecs) * time.Second
	elapsed := Elapsed(t, now)
	if elapsed >= target {
		return 0, true
	}
	return target - elapsed, false
}
