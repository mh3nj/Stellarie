// Package timers implements the Timer System (spec section 21):
// stopwatch, countdown, Pomodoro, interval, focus, project, and custom
// timers. Elapsed/remaining time is always computed fresh from
// started_at/paused_at/accumulated_pause_secs - never cached - the same
// "compute on read" philosophy internal/budgets and internal/goals use,
// so a timer's displayed time is never stale (section 21: "Timers must
// continue working while the main window is hidden" - true here for
// free, since nothing needs to run in the foreground to keep the number
// correct; the only thing that DOES need a background process is
// noticing when a countdown timer's target has been reached, which is
// internal/scheduler's job, not this package's).
package timers

import "time"

// Type is timers.timer_type - one of the values spec section 21 lists.
type Type string

const (
	TypeStopwatch Type = "stopwatch"
	TypeCountdown Type = "countdown"
	TypePomodoro  Type = "pomodoro"
	TypeInterval  Type = "interval"
	TypeFocus     Type = "focus"
	TypeProject   Type = "project"
	TypeCustom    Type = "custom"
)

// ValidType reports whether t is one of the recognized timer types.
func ValidType(t string) bool {
	switch Type(t) {
	case TypeStopwatch, TypeCountdown, TypePomodoro, TypeInterval, TypeFocus, TypeProject, TypeCustom:
		return true
	default:
		return false
	}
}

// Status is timers.status.
type Status string

const (
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
)

// Timer mirrors a timers row.
type Timer struct {
	ID                   int64
	TimerType            Type
	ProjectID            int64 // 0 = no project
	Label                string
	DurationSecs         int64 // 0/absent = no target (stopwatch)
	StartedAt            time.Time
	EndedAt              *time.Time
	Status               Status
	PausedAt             *time.Time
	AccumulatedPauseSecs int64
	MetadataJSON         string // Pomodoro phase state lives here - see pomodoro.go
}

// HasTarget reports whether the timer counts down toward a fixed
// duration (countdown/pomodoro/interval/focus/project/custom all
// commonly do; stopwatch never does - section 21's own list only calls
// stopwatch out as open-ended).
func (t Timer) HasTarget() bool {
	return t.DurationSecs > 0
}
