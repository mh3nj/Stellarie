// Package alarms implements the Alarm System (spec section 22):
// one-time, recurring, relative, activity-based, and schedule alarms.
// Firing decisions are pure functions of stored fields plus "now" (see
// recurrence.go, condition.go, due.go) so internal/scheduler's tests can
// drive them with a fabricated clock and fakes, the same pattern
// internal/activity's Engine uses for Pause/Resume timing.
package alarms

import "time"

// Type is alarms.alarm_type.
type Type string

const (
	TypeOneTime       Type = "one_time"
	TypeRecurring     Type = "recurring"
	TypeRelative      Type = "relative"
	TypeActivityBased Type = "activity_based"
	TypeSchedule      Type = "schedule"
)

func ValidType(t string) bool {
	switch Type(t) {
	case TypeOneTime, TypeRecurring, TypeRelative, TypeActivityBased, TypeSchedule:
		return true
	default:
		return false
	}
}

// Alarm mirrors an alarms row.
type Alarm struct {
	ID               int64
	AlarmType        Type
	Label            string
	FireAt           *time.Time // one_time/relative/schedule
	RecurrenceRule   string     // recurring/schedule - see recurrence.go
	TriggerCondition string     // activity_based - see condition.go
	Enabled          bool
	SnoozeMinutes    int
	Sound            string
	CreatedAt        time.Time
	LastFiredAt      *time.Time
	SnoozedUntil     *time.Time
}
