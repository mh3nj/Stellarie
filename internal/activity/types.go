// Package activity implements the Activity Engine (spec section 6-9):
// turning a stream of raw foreground-window/idle samples into debounced,
// storable activity events. Section 7 is explicit that this must NOT
// write a database row per poll tick - Debouncer below is the piece that
// enforces that, and it's plain Go with zero I/O so it's fully unit
// tested without a database, a display, or a platform.ActiveWindowWatcher.
package activity

import "time"

// Sample is one poll of the current foreground window + idle state.
// Nothing here is ever keystroke or clipboard content - see section 53.
type Sample struct {
	Time        time.Time
	ProcessName string
	WindowTitle string
	Idle        bool
}

// CompletedEvent is a finished, debounced run of continuous activity
// what actually gets persisted to the activity_events table. Multiple
// Samples collapse into one CompletedEvent as long as ProcessName and
// Idle stay the same.
type CompletedEvent struct {
	StartedAt   time.Time
	Duration    time.Duration
	ProcessName string
	WindowTitle string // the most recently observed title during the run
	Idle        bool
}
