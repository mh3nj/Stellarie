package activity

import "time"

// Debouncer collapses a stream of Samples into CompletedEvents,
// coalescing consecutive samples that share (ProcessName, Idle) into a
// single run instead of one row per poll. It holds no reference to a
// database, a clock, or a platform watcher - Feed and Flush are pure
// functions of their arguments and the Debouncer's own prior state, which
// is what makes debouncer_test.go able to exercise every case with
// fabricated timestamps instead of real sleeps.
//
// Not safe for concurrent use - the Activity Engine owns exactly one
// Debouncer and drives it from a single goroutine's poll loop.
type Debouncer struct {
	current *Sample
}

// NewDebouncer returns an empty Debouncer with no run in progress.
func NewDebouncer() *Debouncer {
	return &Debouncer{}
}

// Feed processes one new sample.
//
//   - If nothing was in progress, the sample starts a new run and Feed
//     returns nil.
//   - If the sample continues the current run (same ProcessName and
//     Idle), the run's observed WindowTitle is updated to this sample's
//     title and Feed returns nil.
//   - If the sample represents a change (different ProcessName or Idle
//     state), the prior run is closed out as a CompletedEvent ending at
//     this sample's timestamp, and the new sample starts the next run.
//
// The "closed at this sample's timestamp" choice is a deliberate
// approximation shared by every debounce-based tracker: the true
// transition happened sometime between the previous and current poll,
// and we don't know exactly when. It's why analytics built on this data
// should be presented as estimates (section 66), not as precise
// measurements.
func (d *Debouncer) Feed(s Sample) *CompletedEvent {
	if d.current == nil {
		d.current = &s
		return nil
	}

	if d.current.ProcessName == s.ProcessName && d.current.Idle == s.Idle {
		d.current.WindowTitle = s.WindowTitle
		return nil
	}

	completed := d.buildCompletedEvent(s.Time)
	d.current = &s
	return completed
}

// Flush closes whatever run is in progress as of `at`, for use on
// shutdown, on an explicit pause, or when the Engine detects a large gap
// between polls (e.g. the system slept) and doesn't want that gap
// silently attributed to whatever app happened to be foreground before
// sleeping. Returns nil if nothing was in progress.
func (d *Debouncer) Flush(at time.Time) *CompletedEvent {
	if d.current == nil {
		return nil
	}
	completed := d.buildCompletedEvent(at)
	d.current = nil
	return completed
}

func (d *Debouncer) buildCompletedEvent(endedAt time.Time) *CompletedEvent {
	duration := endedAt.Sub(d.current.Time)
	if duration < 0 {
		// Defensive only - a caller feeding out-of-order timestamps is a
		// bug elsewhere, not something this type should propagate as a
		// negative-duration event into the database.
		duration = 0
	}
	return &CompletedEvent{
		StartedAt:   d.current.Time,
		Duration:    duration,
		ProcessName: d.current.ProcessName,
		WindowTitle: d.current.WindowTitle,
		Idle:        d.current.Idle,
	}
}

// InProgress reports the run currently being accumulated, if any. Used
// for live status (tray label, frontend "Currently: X" display) without
// needing to close the run to see it.
func (d *Debouncer) InProgress() (Sample, bool) {
	if d.current == nil {
		return Sample{}, false
	}
	return *d.current, true
}
