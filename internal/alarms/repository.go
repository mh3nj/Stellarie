package alarms

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository is the SQL-backed store for alarms rows. Concrete struct,
// same reasoning as internal/timers.Repository - the firing DECISION
// (Due, above) is pure and tested without a database; this layer is
// just CRUD plus the handful of state transitions firing requires.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const alarmColumns = `
	id, alarm_type, label, fire_at, recurrence_rule, trigger_condition,
	enabled, snooze_minutes, sound, created_at, last_fired_at, snoozed_until
`

func scanAlarm(row interface{ Scan(...any) error }) (Alarm, error) {
	var a Alarm
	var alarmType, label, recurrenceRule, triggerCondition, sound sql.NullString
	var fireAt, lastFiredAt, snoozedUntil sql.NullInt64
	var enabledInt int
	var createdAtUnix int64

	if err := row.Scan(&a.ID, &alarmType, &label, &fireAt, &recurrenceRule, &triggerCondition,
		&enabledInt, &a.SnoozeMinutes, &sound, &createdAtUnix, &lastFiredAt, &snoozedUntil); err != nil {
		return Alarm{}, err
	}

	a.AlarmType = Type(alarmType.String)
	a.Label = label.String
	a.RecurrenceRule = recurrenceRule.String
	a.TriggerCondition = triggerCondition.String
	a.Enabled = enabledInt != 0
	a.Sound = sound.String
	a.CreatedAt = time.Unix(createdAtUnix, 0)

	if fireAt.Valid {
		when := time.Unix(fireAt.Int64, 0)
		a.FireAt = &when
	}
	if lastFiredAt.Valid {
		when := time.Unix(lastFiredAt.Int64, 0)
		a.LastFiredAt = &when
	}
	if snoozedUntil.Valid {
		when := time.Unix(snoozedUntil.Int64, 0)
		a.SnoozedUntil = &when
	}
	return a, nil
}

// Create adds a new alarm. Exactly one of fireAt (unix seconds, 0 = not
// set)/recurrenceRule/triggerCondition should be meaningful for a given
// alarmType - validated by the caller (AlarmService), which knows the
// per-type rules; this layer just stores what it's given.
func (r *Repository) Create(ctx context.Context, alarmType Type, label string, fireAt int64, recurrenceRule, triggerCondition string, snoozeMinutes int, sound string) (int64, error) {
	if !ValidType(string(alarmType)) {
		return 0, fmt.Errorf("alarms: invalid alarm type %q", alarmType)
	}
	if snoozeMinutes <= 0 {
		snoozeMinutes = 5 // matches the schema's own DEFAULT (section 22)
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO alarms (alarm_type, label, fire_at, recurrence_rule, trigger_condition, enabled, snooze_minutes, sound, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, unixepoch())
	`, string(alarmType), nullableString(label), nullableUnix(fireAt), nullableString(recurrenceRule), nullableString(triggerCondition), snoozeMinutes, nullableString(sound))
	if err != nil {
		return 0, fmt.Errorf("alarms: creating: %w", err)
	}
	return res.LastInsertId()
}

// List returns every alarm, enabled or not, newest first - the Alarms
// page shows both so a disabled alarm can be re-enabled rather than
// disappearing.
func (r *Repository) List(ctx context.Context) ([]Alarm, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+alarmColumns+` FROM alarms ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("alarms: listing: %w", err)
	}
	defer rows.Close()

	var out []Alarm
	for rows.Next() {
		a, err := scanAlarm(rows)
		if err != nil {
			return nil, fmt.Errorf("alarms: scanning row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListEnabled returns only enabled alarms - what internal/scheduler
// actually needs to evaluate on every tick, so it never wastes a Due()
// call (or, for activity-based alarms, a session-aggregate query) on
// something the person turned off.
func (r *Repository) ListEnabled(ctx context.Context) ([]Alarm, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+alarmColumns+` FROM alarms WHERE enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("alarms: listing enabled: %w", err)
	}
	defer rows.Close()

	var out []Alarm
	for rows.Next() {
		a, err := scanAlarm(rows)
		if err != nil {
			return nil, fmt.Errorf("alarms: scanning row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetEnabled toggles an alarm on/off (the Alarms page's own per-alarm
// switch - not the same as Delete, section 22 doesn't ask for silent
// deletion, just the ability to turn one off without losing its config).
func (r *Repository) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE alarms SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("alarms: setting enabled for %d: %w", id, err)
	}
	return nil
}

// Delete removes an alarm entirely.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM alarms WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("alarms: deleting %d: %w", id, err)
	}
	return nil
}

// MarkFired records that an alarm fired at `at`, clearing any snooze
// (section 22's "Dismiss" - firing consumes the snooze that led to it,
// if any) and, for a plain one-shot alarm (no recurrence), disabling it
// so section 22's "Allow dismiss"/one-time-only expectation holds even
// if Due's own LastFiredAt-based one-shot check were ever bypassed by a
// future caller - belt and suspenders on an irreversible-feeling action
// (a fired alarm that silently fires again tomorrow would be a bad
// surprise).
func (r *Repository) MarkFired(ctx context.Context, id int64, at time.Time) error {
	var recurrenceRule sql.NullString
	if err := r.db.QueryRowContext(ctx, `SELECT recurrence_rule FROM alarms WHERE id = ?`, id).Scan(&recurrenceRule); err != nil {
		return fmt.Errorf("alarms: marking %d fired: %w", id, err)
	}

	if recurrenceRule.Valid && recurrenceRule.String != "" {
		_, err := r.db.ExecContext(ctx, `
			UPDATE alarms SET last_fired_at = ?, snoozed_until = NULL WHERE id = ?
		`, at.Unix(), id)
		if err != nil {
			return fmt.Errorf("alarms: marking %d fired: %w", id, err)
		}
		return nil
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE alarms SET last_fired_at = ?, snoozed_until = NULL, enabled = 0 WHERE id = ?
	`, at.Unix(), id)
	if err != nil {
		return fmt.Errorf("alarms: marking %d fired: %w", id, err)
	}
	return nil
}

// MarkFiredRecurring is like MarkFired but never disables the alarm
// used by internal/scheduler specifically for activity-based alarms,
// which have no RecurrenceRule column value (their "recurrence" is just
// "re-evaluate the condition every day") but must still keep firing on
// future days rather than being disabled after the first trigger.
func (r *Repository) MarkFiredRecurring(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE alarms SET last_fired_at = ?, snoozed_until = NULL WHERE id = ?
	`, at.Unix(), id)
	if err != nil {
		return fmt.Errorf("alarms: marking %d fired: %w", id, err)
	}
	return nil
}

// Snooze sets snoozed_until to now+minutes (falling back to the alarm's
// own configured SnoozeMinutes if minutes <= 0) - section 22 "Allow
// snooze".
func (r *Repository) Snooze(ctx context.Context, id int64, now time.Time, minutes int) error {
	if minutes <= 0 {
		if err := r.db.QueryRowContext(ctx, `SELECT snooze_minutes FROM alarms WHERE id = ?`, id).Scan(&minutes); err != nil {
			return fmt.Errorf("alarms: snoozing %d: %w", id, err)
		}
	}
	until := now.Add(time.Duration(minutes) * time.Minute)
	_, err := r.db.ExecContext(ctx, `UPDATE alarms SET snoozed_until = ? WHERE id = ?`, until.Unix(), id)
	if err != nil {
		return fmt.Errorf("alarms: snoozing %d: %w", id, err)
	}
	return nil
}

// Dismiss clears a snooze without treating it as a fire (section 22
// "Allow dismiss" - dismissing just closes the notification, it isn't a
// new firing event).
func (r *Repository) Dismiss(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE alarms SET snoozed_until = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("alarms: dismissing %d: %w", id, err)
	}
	return nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableUnix(secs int64) any {
	if secs == 0 {
		return nil
	}
	return secs
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
