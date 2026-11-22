package alarms

import (
	"testing"
	"time"
)

var dueBase = time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

func ptr(t time.Time) *time.Time { return &t }

func TestDue_DisabledNeverFires(t *testing.T) {
	fireAt := dueBase.Add(-time.Hour)
	a := Alarm{Enabled: false, FireAt: &fireAt}
	due, _ := Due(a, dueBase)
	if due {
		t.Fatal("a disabled alarm must never be due")
	}
}

func TestDue_OneTimeNotYetReached(t *testing.T) {
	fireAt := dueBase.Add(time.Hour)
	a := Alarm{Enabled: true, FireAt: &fireAt}
	due, _ := Due(a, dueBase)
	if due {
		t.Fatal("must not be due before FireAt")
	}
}

func TestDue_OneTimeReached(t *testing.T) {
	fireAt := dueBase.Add(-time.Minute)
	a := Alarm{Enabled: true, FireAt: &fireAt}
	due, fireTime := Due(a, dueBase)
	if !due {
		t.Fatal("must be due once now has passed FireAt")
	}
	if !fireTime.Equal(dueBase) {
		t.Fatalf("fireTime = %v, want %v (now)", fireTime, dueBase)
	}
}

func TestDue_OneTimeAlreadyFiredNeverAgain(t *testing.T) {
	fireAt := dueBase.Add(-time.Hour)
	a := Alarm{Enabled: true, FireAt: &fireAt, LastFiredAt: ptr(dueBase.Add(-30 * time.Minute))}
	due, _ := Due(a, dueBase)
	if due {
		t.Fatal("a one-shot alarm that already fired must never fire again")
	}
}

func TestDue_NoFireAtNoRecurrence(t *testing.T) {
	a := Alarm{Enabled: true}
	due, _ := Due(a, dueBase)
	if due {
		t.Fatal("an alarm with neither FireAt nor a recurrence rule can never be due")
	}
}

func TestDue_RecurringFirstFire(t *testing.T) {
	// Never fired yet - basis is CreatedAt.
	a := Alarm{
		Enabled:        true,
		RecurrenceRule: "FREQ=DAILY;TIME=08:00",
		CreatedAt:      dueBase.Add(-24 * time.Hour), // created yesterday at 09:00
	}
	// "now" is 09:00 today - the next 08:00 after yesterday 09:00 is
	// today 08:00, which has already passed by 09:00 today, so it's due.
	due, _ := Due(a, dueBase)
	if !due {
		t.Fatal("expected the recurring alarm to be due")
	}
}

func TestDue_RecurringNotYetDueToday(t *testing.T) {
	a := Alarm{
		Enabled:        true,
		RecurrenceRule: "FREQ=DAILY;TIME=18:00",
		CreatedAt:      dueBase.Add(-3 * time.Hour), // created earlier this same morning (06:00)
	}
	due, _ := Due(a, dueBase) // now is 09:00, target today is 18:00 - hasn't arrived yet
	if due {
		t.Fatal("must not be due before today's 18:00 occurrence")
	}
}

func TestDue_RecurringUsesLastFiredAsBasis(t *testing.T) {
	// Fired already today at 08:00 - must not fire again until tomorrow's
	// 08:00, even though CreatedAt was long ago.
	a := Alarm{
		Enabled:        true,
		RecurrenceRule: "FREQ=DAILY;TIME=08:00",
		CreatedAt:      dueBase.Add(-30 * 24 * time.Hour),
		LastFiredAt:    ptr(time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)),
	}
	due, _ := Due(a, dueBase) // now is 09:00 same day, already fired at 08:00
	if due {
		t.Fatal("must not re-fire the same day's occurrence")
	}

	dueTomorrow, _ := Due(a, dueBase.Add(22*time.Hour+59*time.Minute)) // 07:59 tomorrow, still before 08:00
	if dueTomorrow {
		t.Fatal("must not be due before tomorrow's 08:00")
	}
	dueTomorrowAt8, _ := Due(a, dueBase.Add(23*time.Hour)) // 08:00 tomorrow exactly
	if !dueTomorrowAt8 {
		t.Fatal("must be due once tomorrow's occurrence has arrived")
	}
}

func TestDue_MalformedRecurrenceRuleNeverCrashesOrFires(t *testing.T) {
	a := Alarm{Enabled: true, RecurrenceRule: "not a valid rule", CreatedAt: dueBase.Add(-time.Hour)}
	due, _ := Due(a, dueBase)
	if due {
		t.Fatal("a malformed rule must be treated as not-due, never crash or spuriously fire")
	}
}

func TestDue_SnoozeOverridesEverythingUntilItArrives(t *testing.T) {
	fireAt := dueBase.Add(-time.Hour) // would normally already be due
	a := Alarm{Enabled: true, FireAt: &fireAt, SnoozedUntil: ptr(dueBase.Add(10 * time.Minute))}
	due, _ := Due(a, dueBase)
	if due {
		t.Fatal("while snoozed, must not be due even though the underlying FireAt has passed")
	}
}

func TestDue_SnoozeFiresOnceItArrives(t *testing.T) {
	fireAt := dueBase.Add(-time.Hour)
	snoozedUntil := dueBase.Add(-time.Minute)
	a := Alarm{Enabled: true, FireAt: &fireAt, SnoozedUntil: &snoozedUntil}
	due, fireTime := Due(a, dueBase)
	if !due {
		t.Fatal("expected the snooze to be due once now reaches SnoozedUntil")
	}
	if !fireTime.Equal(dueBase) {
		t.Fatalf("fireTime = %v, want %v (now)", fireTime, dueBase)
	}
}

func TestDue_SnoozeOnAlreadyFiredOneTimeStillFires(t *testing.T) {
	// A one-time alarm that already fired once, then got snoozed from the
	// notification, must still fire again when the snooze arrives - the
	// "already fired, never again" one-shot rule is about the ORIGINAL
	// schedule, not about a snooze the person explicitly asked for.
	fireAt := dueBase.Add(-2 * time.Hour)
	snoozedUntil := dueBase.Add(-time.Minute)
	a := Alarm{
		Enabled:      true,
		FireAt:       &fireAt,
		LastFiredAt:  ptr(dueBase.Add(-90 * time.Minute)),
		SnoozedUntil: &snoozedUntil,
	}
	due, _ := Due(a, dueBase)
	if !due {
		t.Fatal("a snooze must fire even on an alarm whose original one-shot already fired")
	}
}
