package scheduler

import (
	"context"
	"testing"
	"time"

	"stellarei/internal/activity"
	"stellarei/internal/alarms"
	"stellarei/internal/categories"
	"stellarei/internal/focus"
	"stellarei/internal/notify"
	"stellarei/internal/projects"
	"stellarei/internal/timers"
)

// --- fakes ---------------------------------------------------------------

type fakeTimerStore struct {
	active    []timers.Timer
	completed map[int64]time.Time
	created   []timers.Timer
	nextID    int64
}

func newFakeTimerStore(active ...timers.Timer) *fakeTimerStore {
	return &fakeTimerStore{active: active, completed: map[int64]time.Time{}, nextID: 1000}
}

func (f *fakeTimerStore) ListActive(ctx context.Context) ([]timers.Timer, error) {
	return f.active, nil
}

func (f *fakeTimerStore) Complete(ctx context.Context, id int64, at time.Time) error {
	f.completed[id] = at
	for i := range f.active {
		if f.active[i].ID == id {
			f.active[i].Status = timers.StatusCompleted
		}
	}
	return nil
}

func (f *fakeTimerStore) Create(ctx context.Context, timerType timers.Type, projectID int64, label string, durationSecs int64, metadataJSON string, startedAt time.Time) (int64, error) {
	f.nextID++
	t := timers.Timer{
		ID: f.nextID, TimerType: timerType, ProjectID: projectID, Label: label,
		DurationSecs: durationSecs, MetadataJSON: metadataJSON, StartedAt: startedAt, Status: timers.StatusRunning,
	}
	f.created = append(f.created, t)
	f.active = append(f.active, t)
	return f.nextID, nil
}

type fakeAlarmStore struct {
	enabled        []alarms.Alarm
	fired          []int64
	firedRecurring []int64
}

func (f *fakeAlarmStore) ListEnabled(ctx context.Context) ([]alarms.Alarm, error) {
	return f.enabled, nil
}
func (f *fakeAlarmStore) MarkFired(ctx context.Context, id int64, at time.Time) error {
	f.fired = append(f.fired, id)
	return nil
}
func (f *fakeAlarmStore) MarkFiredRecurring(ctx context.Context, id int64, at time.Time) error {
	f.firedRecurring = append(f.firedRecurring, id)
	return nil
}

type fakeActivitySums struct {
	byCategory map[int64]int64
	byProject  map[int64]int64
}

func (f *fakeActivitySums) SumDurationByCategory(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error) {
	return f.byCategory, nil
}
func (f *fakeActivitySums) SumDurationByProject(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error) {
	return f.byProject, nil
}

type fakeCategoryLookup struct{ cats []categories.Category }

func (f *fakeCategoryLookup) ListCategories(ctx context.Context) ([]categories.Category, error) {
	return f.cats, nil
}

type fakeProjectLookup struct{ projs []projects.Project }

func (f *fakeProjectLookup) ListProjects(ctx context.Context, includeArchived bool) ([]projects.Project, error) {
	return f.projs, nil
}

type notifyCall struct {
	source             notify.Source
	sourceID           int64
	title, body, sound string
}

type fakeNotifier struct{ calls []notifyCall }

func (f *fakeNotifier) Notify(ctx context.Context, source notify.Source, sourceID int64, title, body, sound string) error {
	f.calls = append(f.calls, notifyCall{source, sourceID, title, body, sound})
	return nil
}

type fakeActivityStatus struct{ state activity.State }

func (f *fakeActivityStatus) Status() activity.State { return f.state }

func newScheduler(timerStore TimerStore, alarmStore AlarmStore, sums ActivitySums, cats CategoryLookup, projs ProjectLookup, notifier Notifier) *Scheduler {
	return New(timerStore, alarmStore, sums, cats, projs, notifier, nil, Config{})
}

var schedBase = time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

// --- timer completion ------------------------------------------------------

func TestTick_CountdownTimerCompletesAndNotifies(t *testing.T) {
	tm := timers.Timer{ID: 1, TimerType: timers.TypeCountdown, Label: "Deep work", DurationSecs: 1500, StartedAt: schedBase, Status: timers.StatusRunning}
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	s := newScheduler(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier)

	// Not yet complete at +20 minutes (target is 25).
	s.tick(schedBase.Add(20 * time.Minute))
	if len(timerStore.completed) != 0 {
		t.Fatal("must not complete before the target duration")
	}
	if len(notifier.calls) != 0 {
		t.Fatal("must not notify before completion")
	}

	// Complete at +25 minutes.
	s.tick(schedBase.Add(25 * time.Minute))
	if _, ok := timerStore.completed[1]; !ok {
		t.Fatal("expected timer 1 to be completed")
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected exactly 1 notification, got %d", len(notifier.calls))
	}
	if notifier.calls[0].source != notify.SourceSystem || notifier.calls[0].sourceID != 1 {
		t.Fatalf("unexpected notification: %+v", notifier.calls[0])
	}
}

func TestTick_PausedTimerNeverCompletesEvenPastTarget(t *testing.T) {
	pausedAt := schedBase.Add(10 * time.Minute)
	tm := timers.Timer{
		ID: 1, TimerType: timers.TypeCountdown, DurationSecs: 900, // 15 min target
		StartedAt: schedBase, Status: timers.StatusPaused, PausedAt: &pausedAt,
	}
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	s := newScheduler(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier)

	// Wall-clock time has long passed the target, but the timer is
	// paused (frozen at 10 minutes elapsed, target 15) - must not fire.
	s.tick(schedBase.Add(3 * time.Hour))
	if len(timerStore.completed) != 0 {
		t.Fatal("a paused timer must never auto-complete")
	}
	if len(notifier.calls) != 0 {
		t.Fatal("a paused timer must never notify")
	}
}

func TestTick_StopwatchNeverCompletes(t *testing.T) {
	tm := timers.Timer{ID: 1, TimerType: timers.TypeStopwatch, StartedAt: schedBase, Status: timers.StatusRunning} // DurationSecs 0
	timerStore := newFakeTimerStore(tm)
	s := newScheduler(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, &fakeNotifier{})

	s.tick(schedBase.Add(100 * time.Hour))
	if len(timerStore.completed) != 0 {
		t.Fatal("a stopwatch (no target) must never auto-complete")
	}
}

func TestTick_PomodoroWorkCompletionStartsBreak(t *testing.T) {
	state := timers.PomodoroState{Phase: timers.PhaseWork, Config: timers.NewPomodoroConfig(timers.PomodoroConfig{WorkMinutes: 25, ShortBreakMinutes: 5})}
	metadata, err := timers.EncodePomodoroState(state)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	tm := timers.Timer{ID: 1, TimerType: timers.TypePomodoro, Label: "Serenity", DurationSecs: 1500, MetadataJSON: metadata, StartedAt: schedBase, Status: timers.StatusRunning}
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	s := newScheduler(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier)

	s.tick(schedBase.Add(25 * time.Minute))

	if _, ok := timerStore.completed[1]; !ok {
		t.Fatal("expected the work phase timer to be completed")
	}
	if len(timerStore.created) != 1 {
		t.Fatalf("expected exactly 1 new timer (the break phase), got %d", len(timerStore.created))
	}
	next := timerStore.created[0]
	if next.TimerType != timers.TypePomodoro {
		t.Fatalf("next phase timer type = %q, want pomodoro", next.TimerType)
	}
	if next.DurationSecs != 5*60 {
		t.Fatalf("next phase duration = %ds, want 300s (5 min short break)", next.DurationSecs)
	}
	nextState, err := timers.DecodePomodoroState(next.MetadataJSON)
	if err != nil {
		t.Fatalf("decoding next phase state: %v", err)
	}
	if nextState.Phase != timers.PhaseShortBreak {
		t.Fatalf("next phase = %q, want short_break", nextState.Phase)
	}
	if next.Label != "Serenity" {
		t.Fatalf("next phase label = %q, want carried over from the work session", next.Label)
	}

	// Two notifications total for this tick: the pomodoro's own
	// completion notice is exactly 1 (the new break timer doesn't
	// itself complete this same tick).
	if len(notifier.calls) != 1 {
		t.Fatalf("expected exactly 1 notification, got %d: %+v", len(notifier.calls), notifier.calls)
	}
}

// --- alarms: one-time / recurring -----------------------------------------

func TestTick_OneTimeAlarmFiresAndMarks(t *testing.T) {
	fireAt := schedBase.Add(-time.Minute)
	a := alarms.Alarm{ID: 7, AlarmType: alarms.TypeOneTime, Label: "Stop for dinner", Enabled: true, FireAt: &fireAt, CreatedAt: schedBase.Add(-time.Hour)}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	notifier := &fakeNotifier{}
	s := newScheduler(&fakeTimerStore{}, alarmStore, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier)

	s.tick(schedBase)

	if len(alarmStore.fired) != 1 || alarmStore.fired[0] != 7 {
		t.Fatalf("expected alarm 7 marked fired (one-shot path), got fired=%v firedRecurring=%v", alarmStore.fired, alarmStore.firedRecurring)
	}
	if len(notifier.calls) != 1 || notifier.calls[0].source != notify.SourceAlarm {
		t.Fatalf("unexpected notifications: %+v", notifier.calls)
	}
}

func TestTick_RecurringAlarmNotYetDueDoesNothing(t *testing.T) {
	a := alarms.Alarm{
		ID: 8, AlarmType: alarms.TypeRecurring, Label: "Start development", Enabled: true,
		RecurrenceRule: "FREQ=DAILY;TIME=18:00", CreatedAt: schedBase.Add(-3 * time.Hour),
	}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	notifier := &fakeNotifier{}
	s := newScheduler(&fakeTimerStore{}, alarmStore, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier)

	s.tick(schedBase) // 09:00, target is 18:00 today

	if len(alarmStore.fired) != 0 || len(alarmStore.firedRecurring) != 0 {
		t.Fatal("must not fire before the recurrence's next occurrence")
	}
	if len(notifier.calls) != 0 {
		t.Fatal("must not notify before due")
	}
}

func TestTick_DisabledAlarmNeverFires(t *testing.T) {
	fireAt := schedBase.Add(-time.Hour)
	a := alarms.Alarm{ID: 9, AlarmType: alarms.TypeOneTime, Enabled: false, FireAt: &fireAt}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{}} // ListEnabled itself wouldn't return it - simulate that correctly
	s := newScheduler(&fakeTimerStore{}, alarmStore, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, &fakeNotifier{})
	_ = a

	s.tick(schedBase)
	if len(alarmStore.fired) != 0 {
		t.Fatal("a disabled alarm shouldn't even be in the enabled list, let alone fire")
	}
}

// --- alarms: activity-based ------------------------------------------------

func TestTick_ActivityBasedAlarmFiresWhenThresholdCrossed(t *testing.T) {
	a := alarms.Alarm{
		ID: 10, AlarmType: alarms.TypeActivityBased, Label: "Gaming check-in", Enabled: true,
		TriggerCondition: "category=Gaming;minutes=60", CreatedAt: schedBase.Add(-3 * time.Hour),
	}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	sums := &fakeActivitySums{byCategory: map[int64]int64{42: 61 * 60}} // 61 minutes of category id 42
	cats := &fakeCategoryLookup{cats: []categories.Category{{ID: 42, Name: "Gaming"}}}
	notifier := &fakeNotifier{}
	s := newScheduler(&fakeTimerStore{}, alarmStore, sums, cats, &fakeProjectLookup{}, notifier)

	s.tick(schedBase)

	if len(alarmStore.firedRecurring) != 1 || alarmStore.firedRecurring[0] != 10 {
		t.Fatalf("expected alarm 10 marked fired via the recurring path (re-arms daily), got %v / %v", alarmStore.fired, alarmStore.firedRecurring)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.calls))
	}
}

func TestTick_ActivityBasedAlarmBelowThresholdDoesNotFire(t *testing.T) {
	a := alarms.Alarm{
		ID: 11, AlarmType: alarms.TypeActivityBased, Enabled: true,
		TriggerCondition: "category=Gaming;minutes=60", CreatedAt: schedBase.Add(-3 * time.Hour),
	}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	sums := &fakeActivitySums{byCategory: map[int64]int64{42: 30 * 60}} // only 30 minutes
	cats := &fakeCategoryLookup{cats: []categories.Category{{ID: 42, Name: "Gaming"}}}
	s := newScheduler(&fakeTimerStore{}, alarmStore, sums, cats, &fakeProjectLookup{}, &fakeNotifier{})

	s.tick(schedBase)

	if len(alarmStore.firedRecurring) != 0 {
		t.Fatal("must not fire below the configured threshold")
	}
}

func TestTick_ActivityBasedAlarmDoesNotDoubleFireSameDay(t *testing.T) {
	firedEarlierToday := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	a := alarms.Alarm{
		ID: 12, AlarmType: alarms.TypeActivityBased, Enabled: true,
		TriggerCondition: "category=Gaming;minutes=60", CreatedAt: schedBase.Add(-30 * 24 * time.Hour),
		LastFiredAt: &firedEarlierToday,
	}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	sums := &fakeActivitySums{byCategory: map[int64]int64{42: 120 * 60}} // well over threshold
	cats := &fakeCategoryLookup{cats: []categories.Category{{ID: 42, Name: "Gaming"}}}
	s := newScheduler(&fakeTimerStore{}, alarmStore, sums, cats, &fakeProjectLookup{}, &fakeNotifier{})

	s.tick(schedBase) // same day as firedEarlierToday

	if len(alarmStore.firedRecurring) != 0 {
		t.Fatal("must not fire again the same local day it already fired")
	}
}

func TestTick_ActivityBasedAlarmReArmsNextDay(t *testing.T) {
	firedYesterday := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	a := alarms.Alarm{
		ID: 13, AlarmType: alarms.TypeActivityBased, Enabled: true,
		TriggerCondition: "category=Gaming;minutes=60", CreatedAt: schedBase.Add(-30 * 24 * time.Hour),
		LastFiredAt: &firedYesterday,
	}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	sums := &fakeActivitySums{byCategory: map[int64]int64{42: 90 * 60}}
	cats := &fakeCategoryLookup{cats: []categories.Category{{ID: 42, Name: "Gaming"}}}
	s := newScheduler(&fakeTimerStore{}, alarmStore, sums, cats, &fakeProjectLookup{}, &fakeNotifier{})

	s.tick(schedBase) // today (Aug 16), different local day than LastFiredAt (Aug 15)

	if len(alarmStore.firedRecurring) != 1 {
		t.Fatal("expected the alarm to re-arm and fire again on a new day")
	}
}

func TestTick_ActivityBasedAlarmUnknownCategoryNameIsIgnored(t *testing.T) {
	a := alarms.Alarm{
		ID: 14, AlarmType: alarms.TypeActivityBased, Enabled: true,
		TriggerCondition: "category=NoSuchCategory;minutes=1", CreatedAt: schedBase.Add(-time.Hour),
	}
	alarmStore := &fakeAlarmStore{enabled: []alarms.Alarm{a}}
	cats := &fakeCategoryLookup{cats: []categories.Category{{ID: 1, Name: "Development"}}}
	s := newScheduler(&fakeTimerStore{}, alarmStore, &fakeActivitySums{}, cats, &fakeProjectLookup{}, &fakeNotifier{})

	s.tick(schedBase) // must not panic or fire on an unresolvable category name

	if len(alarmStore.firedRecurring) != 0 {
		t.Fatal("an alarm referencing a nonexistent category must never fire")
	}
}

// --- OnAlarmFired / OnTimerCompleted callbacks ------------------------------

func TestTick_CallbacksInvoked(t *testing.T) {
	tm := timers.Timer{ID: 1, TimerType: timers.TypeCountdown, DurationSecs: 60, StartedAt: schedBase, Status: timers.StatusRunning}
	fireAt := schedBase.Add(-time.Minute)
	a := alarms.Alarm{ID: 2, AlarmType: alarms.TypeOneTime, Enabled: true, FireAt: &fireAt}

	var timerCompletedCalled, alarmFiredCalled bool
	s := New(
		newFakeTimerStore(tm),
		&fakeAlarmStore{enabled: []alarms.Alarm{a}},
		&fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{},
		&fakeNotifier{},
		nil,
		Config{
			OnTimerCompleted: func(timers.Timer) { timerCompletedCalled = true },
			OnAlarmFired:     func(alarms.Alarm) { alarmFiredCalled = true },
		},
	)

	s.tick(schedBase.Add(time.Minute)) // past the 60s timer target

	if !timerCompletedCalled {
		t.Fatal("expected OnTimerCompleted to be called")
	}
	if !alarmFiredCalled {
		t.Fatal("expected OnAlarmFired to be called")
	}
}

// --- audible alert (Config.PlaySound) ---------------------------------------

// A timer running out and an alarm coming due must both make a noise: the
// OS notification is not something STELLAREI can rely on (Windows DND
// silences the toast's own sound, macOS drops the banner outright when
// authorization was denied), so the tone is played by the app itself.
func TestTick_PlaysSoundForTimerAndAlarm(t *testing.T) {
	tm := timers.Timer{ID: 1, TimerType: timers.TypeCountdown, DurationSecs: 60, StartedAt: schedBase, Status: timers.StatusRunning}
	fireAt := schedBase.Add(-time.Minute)
	a := alarms.Alarm{ID: 2, AlarmType: alarms.TypeOneTime, Enabled: true, FireAt: &fireAt}

	played := 0
	s := New(
		newFakeTimerStore(tm),
		&fakeAlarmStore{enabled: []alarms.Alarm{a}},
		&fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{},
		&fakeNotifier{},
		nil,
		Config{PlaySound: func() { played++ }},
	)

	s.tick(schedBase.Add(time.Minute)) // past the 60s timer target and the alarm's FireAt

	if played != 2 {
		t.Fatalf("PlaySound called %d times, want 2 (one per fired event)", played)
	}
}

// An alarm the person explicitly marked silent must not beep - that
// setting exists precisely for alarms that should only be seen.
func TestTick_SilentAlarmStaysSilent(t *testing.T) {
	fireAt := schedBase.Add(-time.Minute)
	a := alarms.Alarm{ID: 3, AlarmType: alarms.TypeOneTime, Enabled: true, FireAt: &fireAt, Sound: "silent"}

	played := 0
	notifier := &fakeNotifier{}
	s := New(
		&fakeTimerStore{},
		&fakeAlarmStore{enabled: []alarms.Alarm{a}},
		&fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{},
		notifier,
		nil,
		Config{PlaySound: func() { played++ }},
	)

	s.tick(schedBase)

	if len(notifier.calls) != 1 {
		t.Fatalf("expected the silent alarm to still be notified once, got %d notifications", len(notifier.calls))
	}
	if played != 0 {
		t.Fatalf("a silent alarm played a sound %d times, want 0", played)
	}
}

// A nil hook has to be safe: every other test in this file constructs a
// Scheduler with Config{} and ticks it, which is exactly the no-sound
// configuration a headless/server build would use.
func TestTick_NilPlaySoundIsSafe(t *testing.T) {
	tm := timers.Timer{ID: 1, TimerType: timers.TypeCountdown, DurationSecs: 1, StartedAt: schedBase, Status: timers.StatusRunning}
	s := newScheduler(newFakeTimerStore(tm), &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, &fakeNotifier{})

	s.tick(schedBase.Add(time.Minute))
}

// --- focus-mode blocked-app violations --------------------------------------

func focusTimerWith(t *testing.T, blockedApps []string) timers.Timer {
	t.Helper()
	cfg := focus.Config{BlockedApps: blockedApps}
	metadata, err := focus.EncodeConfig(cfg)
	if err != nil {
		t.Fatalf("encoding focus config: %v", err)
	}
	return timers.Timer{ID: 5, TimerType: timers.TypeFocus, Label: "Serenity", ProjectID: 1, DurationSecs: 1500, StartedAt: schedBase, Status: timers.StatusRunning, MetadataJSON: metadata}
}

func TestTick_FocusViolationNotifiesOnce(t *testing.T) {
	tm := focusTimerWith(t, []string{"Steam.exe"})
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	activityStatus := &fakeActivityStatus{state: activity.State{ProcessName: "Steam.exe"}}
	s := New(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier, activityStatus, Config{})

	s.tick(schedBase.Add(time.Minute))
	if len(notifier.calls) != 1 {
		t.Fatalf("expected exactly 1 violation notification, got %d", len(notifier.calls))
	}
	if notifier.calls[0].title != "Focus interrupted" {
		t.Fatalf("unexpected notification: %+v", notifier.calls[0])
	}

	// Still in the blocked app on the next tick - must NOT notify again
	// for the same continuous streak.
	s.tick(schedBase.Add(2 * time.Minute))
	if len(notifier.calls) != 1 {
		t.Fatalf("expected still exactly 1 notification (no re-notify mid-streak), got %d", len(notifier.calls))
	}
}

func TestTick_FocusViolationRenotifiesAfterLeavingAndReturning(t *testing.T) {
	tm := focusTimerWith(t, []string{"Steam.exe"})
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	activityStatus := &fakeActivityStatus{state: activity.State{ProcessName: "Steam.exe"}}
	s := New(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier, activityStatus, Config{})

	s.tick(schedBase.Add(time.Minute)) // enters blocked app -> 1 notification
	activityStatus.state = activity.State{ProcessName: "Code.exe"}
	s.tick(schedBase.Add(2 * time.Minute)) // back on task
	activityStatus.state = activity.State{ProcessName: "Steam.exe"}
	s.tick(schedBase.Add(3 * time.Minute)) // blocked app again -> new streak

	if len(notifier.calls) != 2 {
		t.Fatalf("expected 2 notifications (one per streak), got %d", len(notifier.calls))
	}
}

func TestTick_FocusNoViolationWhenNotInBlockedApp(t *testing.T) {
	tm := focusTimerWith(t, []string{"Steam.exe"})
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	activityStatus := &fakeActivityStatus{state: activity.State{ProcessName: "Code.exe"}}
	s := New(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier, activityStatus, Config{})

	s.tick(schedBase.Add(time.Minute))
	if len(notifier.calls) != 0 {
		t.Fatalf("expected 0 notifications when not in a blocked app, got %d", len(notifier.calls))
	}
}

func TestTick_FocusNoBlockedAppsConfiguredNeverViolates(t *testing.T) {
	tm := focusTimerWith(t, nil) // no blocklist configured
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	activityStatus := &fakeActivityStatus{state: activity.State{ProcessName: "Steam.exe"}}
	s := New(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier, activityStatus, Config{})

	s.tick(schedBase.Add(time.Minute))
	if len(notifier.calls) != 0 {
		t.Fatal("a focus session with no configured blocklist must never produce a violation notification")
	}
}

func TestTick_FocusViolationIgnoredWhenIdleOrPaused(t *testing.T) {
	tm := focusTimerWith(t, []string{"Steam.exe"})
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	activityStatus := &fakeActivityStatus{state: activity.State{ProcessName: "Steam.exe", Idle: true}}
	s := New(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier, activityStatus, Config{})

	s.tick(schedBase.Add(time.Minute))
	if len(notifier.calls) != 0 {
		t.Fatal("an idle foreground process name must not count as a violation")
	}
}

func TestTick_FocusViolationSkippedForPausedFocusTimer(t *testing.T) {
	tm := focusTimerWith(t, []string{"Steam.exe"})
	tm.Status = timers.StatusPaused
	pausedAt := schedBase
	tm.PausedAt = &pausedAt
	timerStore := newFakeTimerStore(tm)
	notifier := &fakeNotifier{}
	activityStatus := &fakeActivityStatus{state: activity.State{ProcessName: "Steam.exe"}}
	s := New(timerStore, &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, notifier, activityStatus, Config{})

	s.tick(schedBase.Add(time.Minute))
	if len(notifier.calls) != 0 {
		t.Fatal("a paused focus session must not trigger violation notifications")
	}
}

func TestTick_NilActivityStatusNeverPanics(t *testing.T) {
	tm := focusTimerWith(t, []string{"Steam.exe"})
	s := newScheduler(newFakeTimerStore(tm), &fakeAlarmStore{}, &fakeActivitySums{}, &fakeCategoryLookup{}, &fakeProjectLookup{}, &fakeNotifier{})
	s.tick(schedBase.Add(time.Minute)) // must not panic with a nil ActivityStatus
}
