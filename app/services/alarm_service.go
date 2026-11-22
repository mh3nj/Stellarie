package services

import (
	"context"
	"fmt"
	"time"

	"stellarei/internal/alarms"
)

// AlarmView is an alarm shaped for the frontend - unix-second timestamps
// instead of *time.Time, and a human-readable NextFireAt the person can
// see without re-implementing recurrence math in TypeScript.
type AlarmView struct {
	ID               int64  `json:"id"`
	AlarmType        string `json:"alarmType"`
	Label            string `json:"label"`
	FireAt           int64  `json:"fireAt,omitempty"` // one_time/relative/schedule
	RecurrenceRule   string `json:"recurrenceRule,omitempty"`
	TriggerCondition string `json:"triggerCondition,omitempty"`
	Enabled          bool   `json:"enabled"`
	SnoozeMinutes    int    `json:"snoozeMinutes"`
	Sound            string `json:"sound,omitempty"`
	CreatedAt        int64  `json:"createdAt"`
	LastFiredAt      int64  `json:"lastFiredAt,omitempty"`
	SnoozedUntil     int64  `json:"snoozedUntil,omitempty"`
	// NextFireAt is the next instant this alarm is expected to fire, best
	// effort - computed the same way internal/scheduler decides
	// "due" (see internal/alarms.Due/Recurrence.NextFireAfter), evaluated
	// against `now` at request time. 0 for a disabled alarm or an
	// activity_based alarm (which doesn't have a single next-instant
	// it's a running total against a per-day threshold, better shown as
	// today's progress; see AlarmService.ActivityProgress).
	NextFireAt int64 `json:"nextFireAt,omitempty"`
}

// AlarmService is bound to the frontend via application.NewService in
// main.go. Thin adapter over internal/alarms.Repository, same shape as
// TimerService - the firing decision logic lives in internal/alarms and
// internal/scheduler, not here.
type AlarmService struct {
	repo *alarms.Repository
}

func NewAlarmService(repo *alarms.Repository) *AlarmService {
	return &AlarmService{repo: repo}
}

func toAlarmView(a alarms.Alarm, now time.Time) AlarmView {
	v := AlarmView{
		ID: a.ID, AlarmType: string(a.AlarmType), Label: a.Label,
		RecurrenceRule: a.RecurrenceRule, TriggerCondition: a.TriggerCondition,
		Enabled: a.Enabled, SnoozeMinutes: a.SnoozeMinutes, Sound: a.Sound,
		CreatedAt: a.CreatedAt.Unix(),
	}
	if a.FireAt != nil {
		v.FireAt = a.FireAt.Unix()
	}
	if a.LastFiredAt != nil {
		v.LastFiredAt = a.LastFiredAt.Unix()
	}
	if a.SnoozedUntil != nil {
		v.SnoozedUntil = a.SnoozedUntil.Unix()
	}

	if a.Enabled && a.AlarmType != alarms.TypeActivityBased {
		if a.RecurrenceRule != "" {
			if rule, err := alarms.ParseRecurrenceRule(a.RecurrenceRule); err == nil {
				basis := a.CreatedAt
				if a.LastFiredAt != nil {
					basis = *a.LastFiredAt
				}
				if basis.Before(now) {
					basis = now.Add(-time.Second) // NextFireAfter wants "strictly after" - don't report a past occurrence as next
				}
				v.NextFireAt = rule.NextFireAfter(basis).Unix()
			}
		} else if a.FireAt != nil && a.LastFiredAt == nil {
			v.NextFireAt = a.FireAt.Unix()
		}
	}
	return v
}

// List returns every alarm (enabled and disabled), newest first.
func (s *AlarmService) List() ([]AlarmView, error) {
	rows, err := s.repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]AlarmView, 0, len(rows))
	for _, a := range rows {
		out = append(out, toAlarmView(a, now))
	}
	return out, nil
}

// CreateOneTime creates a one_time alarm firing at the given unix
// timestamp (section 22 "One-time alarms").
func (s *AlarmService) CreateOneTime(label string, fireAtUnix int64, snoozeMinutes int, sound string) (int64, error) {
	if fireAtUnix <= 0 {
		return 0, fmt.Errorf("alarms: fireAt is required for a one-time alarm")
	}
	return s.repo.Create(context.Background(), alarms.TypeOneTime, label, fireAtUnix, "", "", snoozeMinutes, sound)
}

// CreateRelative creates a "relative" alarm firing minutesFromNow from
// now (section 22 "Relative alarms" - "45 minutes from now"). Stored as
// a plain FireAt just like one_time; the alarm_type is what distinguishes
// how the frontend labels it, not how it's scheduled.
func (s *AlarmService) CreateRelative(label string, minutesFromNow int, snoozeMinutes int, sound string) (int64, error) {
	if minutesFromNow <= 0 {
		return 0, fmt.Errorf("alarms: minutesFromNow must be positive")
	}
	fireAt := time.Now().Add(time.Duration(minutesFromNow) * time.Minute).Unix()
	return s.repo.Create(context.Background(), alarms.TypeRelative, label, fireAt, "", "", snoozeMinutes, sound)
}

// CreateRecurring creates a "recurring" alarm (section 22 - "Every
// weekday 09:00") or a "schedule" alarm (section 22 - "Start development
// session 10:00", which is the same underlying mechanism with a
// different label/intent). recurrenceRule must parse via
// internal/alarms.ParseRecurrenceRule (e.g. "FREQ=DAILY;TIME=09:00" or
// "FREQ=WEEKDAYS;TIME=09:00") - validated here so a malformed rule is
// rejected at save time with a clear error, not silently ignored later
// by the scheduler.
func (s *AlarmService) CreateRecurring(alarmType, label, recurrenceRule string, snoozeMinutes int, sound string) (int64, error) {
	if alarms.Type(alarmType) != alarms.TypeRecurring && alarms.Type(alarmType) != alarms.TypeSchedule {
		return 0, fmt.Errorf("alarms: CreateRecurring wants type \"recurring\" or \"schedule\", got %q", alarmType)
	}
	if _, err := alarms.ParseRecurrenceRule(recurrenceRule); err != nil {
		return 0, err
	}
	return s.repo.Create(context.Background(), alarms.Type(alarmType), label, 0, recurrenceRule, "", snoozeMinutes, sound)
}

// CreateActivityBased creates an "activity_based" alarm (section 22
// "After 1 hour of gaming"). triggerCondition must parse via
// internal/alarms.ParseTriggerCondition (e.g. "category=Gaming;minutes=60"),
// validated here for the same reason as CreateRecurring.
func (s *AlarmService) CreateActivityBased(label, triggerCondition string, snoozeMinutes int, sound string) (int64, error) {
	if _, err := alarms.ParseTriggerCondition(triggerCondition); err != nil {
		return 0, err
	}
	return s.repo.Create(context.Background(), alarms.TypeActivityBased, label, 0, "", triggerCondition, snoozeMinutes, sound)
}

func (s *AlarmService) SetEnabled(id int64, enabled bool) error {
	return s.repo.SetEnabled(context.Background(), id, enabled)
}

func (s *AlarmService) Delete(id int64) error {
	return s.repo.Delete(context.Background(), id)
}

// Snooze snoozes an alarm for minutes (section 22 "Allow snooze") - 0
// falls back to the alarm's own configured SnoozeMinutes.
func (s *AlarmService) Snooze(id int64, minutes int) error {
	return s.repo.Snooze(context.Background(), id, time.Now(), minutes)
}

// Dismiss clears an in-progress snooze without it counting as a firing
// (section 22 "Allow dismiss").
func (s *AlarmService) Dismiss(id int64) error {
	return s.repo.Dismiss(context.Background(), id)
}
