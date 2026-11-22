// Package scheduler is the background component that makes Timers and
// Alarms actually DO something over time - it's the Phase 5 analogue of
// internal/activity.Engine: a mutex-guarded struct with a ticker loop,
// an injectable Clock for testability, and a set of narrow interfaces
// (rather than depending on concrete internal/timers.Repository etc.
// directly) so tick() can be unit tested with fakes and a fabricated
// clock, exactly like engine_test.go does for the Activity Engine.
//
// What it does every tick:
//   - Notices a running countdown/Pomodoro/interval/focus/project/custom
//     timer has reached its target duration, marks it completed, and
//     fires a notification. A completed Pomodoro work/break phase
//     automatically starts the next phase's timer (section 21).
//   - Notices a one_time/relative/schedule alarm's FireAt has passed, or
//     a recurring alarm's next scheduled occurrence has arrived, or a
//     snoozed alarm's snooze has elapsed - fires a notification and
//     records that it fired.
//   - Notices an activity_based alarm's threshold has been crossed for
//     today (e.g. "1 hour of Gaming") - fires once per day, re-arming at
//     local midnight.
//
// Stopwatch timers are deliberately never touched here - they have no
// target, so "noticing completion" isn't a meaningful concept for them;
// they only ever change state via an explicit Stop/Complete call from
// the frontend.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"stellarei/internal/activity"
	"stellarei/internal/alarms"
	"stellarei/internal/categories"
	"stellarei/internal/focus"
	"stellarei/internal/notify"
	"stellarei/internal/projects"
	"stellarei/internal/timers"
)

// TimerStore is the subset of internal/timers.Repository the Scheduler
// needs. Satisfied by *timers.Repository without any adapter code.
type TimerStore interface {
	ListActive(ctx context.Context) ([]timers.Timer, error)
	Complete(ctx context.Context, id int64, at time.Time) error
	Create(ctx context.Context, timerType timers.Type, projectID int64, label string, durationSecs int64, metadataJSON string, startedAt time.Time) (int64, error)
}

// AlarmStore is the subset of internal/alarms.Repository the Scheduler
// needs. Satisfied by *alarms.Repository without any adapter code.
type AlarmStore interface {
	ListEnabled(ctx context.Context) ([]alarms.Alarm, error)
	MarkFired(ctx context.Context, id int64, at time.Time) error
	MarkFiredRecurring(ctx context.Context, id int64, at time.Time) error
}

// ActivitySums is the subset of internal/sessions.Repository the
// Scheduler needs to evaluate activity_based alarms. That interface
// already has both these methods (Analytics/Budgets use them too), so a
// real *sessions.Repository satisfies this with no adapter.
type ActivitySums interface {
	SumDurationByCategory(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error)
	SumDurationByProject(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error)
}

// CategoryLookup/ProjectLookup resolve a trigger condition's human-typed
// NAME (spec section 22's own example, "category=Gaming") to the numeric
// ID sessions are actually stored against. Satisfied by
// *categories.Repository / *projects.Repository directly.
type CategoryLookup interface {
	ListCategories(ctx context.Context) ([]categories.Category, error)
}
type ProjectLookup interface {
	ListProjects(ctx context.Context, includeArchived bool) ([]projects.Project, error)
}

// ActivityStatus is the subset of internal/activity.Engine the
// Scheduler needs to detect a Focus Mode blocked-app violation (spec
// section 20's "optional blocked applications") - satisfied by
// *activity.Engine directly. Optional: a nil ActivityStatus (the
// zero value passed to New) simply skips focus-violation checking
// entirely, the same nil-is-fine contract
// activity.Config.ProjectResolver/CategoryResolver already use, so a
// caller that doesn't care about Focus Mode never has to construct one.
type ActivityStatus interface {
	Status() activity.State
}

// Notifier sends a notification and logs it to history in one call
// satisfied by *notify.Notifier. A separate, smaller interface here
// (rather than importing notify.Notifier's concrete type as a field)
// only for the same test-fakeability reason as everything else in this
// file.
type Notifier interface {
	Notify(ctx context.Context, source notify.Source, sourceID int64, title, body, sound string) error
}

// Config controls Scheduler behavior. Zero-value fields fall back to
// sane defaults in New, matching internal/activity.Config's pattern.
type Config struct {
	// TickInterval mirrors activity.Config.PollInterval's reasoning
	// (section 50's near-zero-idle-CPU budget): checking timers/alarms
	// every few seconds is more than precise enough for anything a human
	// perceives as "on time".
	TickInterval time.Duration

	// Clock returns the current time. Defaults to time.Now in New.
	// Overridable so tick() can be tested with a fabricated clock - see
	// scheduler_test.go.
	Clock func() time.Time

	// OnAlarmFired/OnTimerCompleted, if set, are called synchronously
	// from tick() after the corresponding DB state change and
	// notification - main.go wires these to emit Wails events so the
	// frontend can refresh without polling. Must not block.
	OnAlarmFired     func(alarms.Alarm)
	OnTimerCompleted func(timers.Timer)

	// PlaySound, if set, is called the moment a timer completes or an
	// alarm fires, and is how STELLAREI makes an audible noise that does
	// NOT depend on the OS notification pipeline - a muted/DND/'do not
	// disturb' Windows session silences a toast's own sound while still
	// accepting the toast, and macOS drops it entirely if notification
	// authorization was denied. Neither is detectable from here, so the
	// tone is played by the app itself (internal/alert).
	//
	// Injected rather than imported directly for the same reason
	// everything else in this Config is: scheduler_test.go must never
	// play audio on the machine running the tests, and the Scheduler
	// stays decoupled from the audio backend exactly as it is from
	// Wails events. Nil means the notification paths are visual-only.
	PlaySound func()
}

const defaultTickInterval = 5 * time.Second

// Scheduler is the background timers/alarms engine. All exported methods
// are safe for concurrent use.
type Scheduler struct {
	timerStore     TimerStore
	alarmStore     AlarmStore
	activitySums   ActivitySums
	categoryLookup CategoryLookup
	projectLookup  ProjectLookup
	notifier       Notifier
	activityStatus ActivityStatus // optional - nil skips focus-violation checking, see ActivityStatus's doc comment

	cfg Config

	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc

	// focusWarned tracks, per running focus timer, whether a
	// blocked-app violation has already been notified for the CURRENT
	// continuous streak of being in a blocked app - see
	// checkFocusViolations. Transient, in-memory, never persisted:
	// losing it on restart just means one possible extra/missing
	// warning around a restart, never incorrect timer/alarm state (the
	// durable state that actually matters).
	focusWarned map[int64]bool
}

func New(timerStore TimerStore, alarmStore AlarmStore, activitySums ActivitySums, categoryLookup CategoryLookup, projectLookup ProjectLookup, notifier Notifier, activityStatus ActivityStatus, cfg Config) *Scheduler {
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = defaultTickInterval
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Scheduler{
		timerStore:     timerStore,
		alarmStore:     alarmStore,
		activitySums:   activitySums,
		categoryLookup: categoryLookup,
		projectLookup:  projectLookup,
		notifier:       notifier,
		activityStatus: activityStatus,
		cfg:            cfg,
		focusWarned:    map[int64]bool{},
	}
}

// Start launches the tick loop in a background goroutine and returns
// immediately, matching internal/activity.Engine.Start's contract.
func (s *Scheduler) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go s.run(runCtx)
}

// Stop cancels the tick loop and blocks until it exits - no goroutine
// leak, per section 51.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cfg.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}

// tick is the entire per-poll decision, called from the real ticker loop
// and directly from tests with a fabricated `now` - same pattern as
// activity.Engine.tick.
func (s *Scheduler) tick(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.checkTimers(ctx, now)
	s.checkAlarms(ctx, now)
	s.checkFocusViolations(ctx, now)
}

func (s *Scheduler) checkTimers(ctx context.Context, now time.Time) {
	active, err := s.timerStore.ListActive(ctx)
	if err != nil {
		log.Printf("scheduler: listing active timers: %v", err)
		return
	}

	for _, t := range active {
		if t.Status != timers.StatusRunning {
			continue // a paused timer's target isn't "reached" - see internal/timers.Elapsed
		}
		if _, complete := timers.Remaining(t, now); !complete {
			continue
		}

		if err := s.timerStore.Complete(ctx, t.ID, now); err != nil {
			log.Printf("scheduler: completing timer %d: %v", t.ID, err)
			continue
		}

		title, body := timerCompletionMessage(t)
		// Audible first, and unconditionally: a timer reaching zero has
		// happened whether or not the OS is willing to show a banner for
		// it, and this is the half of "time is up" a person actually
		// notices while looking somewhere else.
		if s.cfg.PlaySound != nil {
			s.cfg.PlaySound()
		}
		if err := s.notifier.Notify(ctx, notify.SourceSystem, t.ID, title, body, ""); err != nil {
			log.Printf("scheduler: notifying timer %d completion: %v", t.ID, err)
			// Section 56: a failed notification must not stop the rest
			// of this timer's completion handling (Pomodoro chaining
			// below still needs to run) or the next timer's check.
		}
		if s.cfg.OnTimerCompleted != nil {
			s.cfg.OnTimerCompleted(t)
		}

		if t.TimerType == timers.TypePomodoro {
			s.advancePomodoro(ctx, t, now)
		}
	}
}

// advancePomodoro starts the NEXT phase's timer once a Pomodoro phase
// completes (section 21) - a work session finishing automatically
// starts a break, and vice versa, so the person doesn't have to manually
// re-trigger each phase.
func (s *Scheduler) advancePomodoro(ctx context.Context, completed timers.Timer, now time.Time) {
	state, err := timers.DecodePomodoroState(completed.MetadataJSON)
	if err != nil {
		log.Printf("scheduler: decoding pomodoro state for timer %d: %v", completed.ID, err)
		return
	}
	next := timers.NextPhase(state)
	metadataJSON, err := timers.EncodePomodoroState(next)
	if err != nil {
		log.Printf("scheduler: encoding next pomodoro phase for timer %d: %v", completed.ID, err)
		return
	}
	duration := next.DurationFor(next.Phase)
	if _, err := s.timerStore.Create(ctx, timers.TypePomodoro, completed.ProjectID, completed.Label, int64(duration.Seconds()), metadataJSON, now); err != nil {
		log.Printf("scheduler: starting next pomodoro phase after timer %d: %v", completed.ID, err)
	}
}

func timerCompletionMessage(t timers.Timer) (title, body string) {
	label := t.Label
	if label == "" {
		label = string(t.TimerType)
	}

	if t.TimerType == timers.TypePomodoro {
		if state, err := timers.DecodePomodoroState(t.MetadataJSON); err == nil {
			switch state.Phase {
			case timers.PhaseWork:
				return "Focus session complete", "Time for a break - " + label
			default:
				return "Break's over", "Back to work - " + label
			}
		}
	}
	return "Timer complete", label
}

// checkFocusViolations implements the notification half of Focus Mode's
// "optional blocked applications" (spec section 20): if a running focus
// timer has a blocked-apps list and the person's current foreground app
// (per ActivityStatus, real activity data - never invented) is on it,
// send exactly one notification per continuous streak of being in a
// blocked app, not one every tick - nobody wants a notification every 5
// seconds for as long as a blocked app stays focused.
func (s *Scheduler) checkFocusViolations(ctx context.Context, now time.Time) {
	if s.activityStatus == nil {
		return // no engine wired in - see ActivityStatus's doc comment
	}

	active, err := s.timerStore.ListActive(ctx)
	if err != nil {
		log.Printf("scheduler: listing active timers for focus-violation check: %v", err)
		return
	}

	state := s.activityStatus.Status()

	stillFocusing := map[int64]bool{}
	for _, t := range active {
		if t.TimerType != timers.TypeFocus || t.Status != timers.StatusRunning {
			continue
		}
		stillFocusing[t.ID] = true

		cfg, err := focus.DecodeConfig(t.MetadataJSON)
		if err != nil || len(cfg.BlockedApps) == 0 {
			continue
		}

		blocked := !state.Idle && !state.Paused && focus.IsBlockedApp(state.ProcessName, cfg.BlockedApps)
		if !blocked {
			delete(s.focusWarned, t.ID) // left the blocked app (or went idle/paused) - a fresh warning fires next time they return to one
			continue
		}
		if s.focusWarned[t.ID] {
			continue // already warned for this continuous streak
		}
		s.focusWarned[t.ID] = true

		label := t.Label
		if label == "" {
			label = "your focus session"
		}
		// Audible too: a blocked app opened in front of the person is
		// waited for visually from another window, so the warning has to
		// arrive as sound as well (same reasoning as timer completion).
		if s.cfg.PlaySound != nil {
			s.cfg.PlaySound()
		}
		if err := s.notifier.Notify(ctx, notify.SourceSystem, t.ID, "Focus interrupted",
			fmt.Sprintf("%s is on your blocked list during %s.", state.ProcessName, label), ""); err != nil {
			log.Printf("scheduler: notifying focus violation for timer %d: %v", t.ID, err)
		}
	}

	// Prune any tracked timer that's no longer an active, running focus
	// session (finished/cancelled/paused) - otherwise focusWarned would
	// grow forever across a long-running process (section 51's "no
	// goroutine leak" spirit applies to unbounded in-memory maps too).
	for id := range s.focusWarned {
		if !stillFocusing[id] {
			delete(s.focusWarned, id)
		}
	}
}

func (s *Scheduler) checkAlarms(ctx context.Context, now time.Time) {
	enabled, err := s.alarmStore.ListEnabled(ctx)
	if err != nil {
		log.Printf("scheduler: listing enabled alarms: %v", err)
		return
	}

	// Built lazily (nil until first needed) since most ticks won't have
	// any activity_based alarms to evaluate, and every tick rebuilding
	// these would mean a categories/projects query the vast majority of
	// alarms never use.
	var categoryByName, projectByName map[string]int64

	for _, a := range enabled {
		if a.AlarmType == alarms.TypeActivityBased {
			if categoryByName == nil {
				categoryByName = s.buildCategoryNameMap(ctx)
			}
			if projectByName == nil {
				projectByName = s.buildProjectNameMap(ctx)
			}
			s.checkActivityAlarm(ctx, a, now, categoryByName, projectByName)
			continue
		}

		due, fireAt := alarms.Due(a, now)
		if !due {
			continue
		}
		s.notifyAndMark(ctx, a, fireAt, false)
	}
}

func (s *Scheduler) buildCategoryNameMap(ctx context.Context) map[string]int64 {
	out := map[string]int64{}
	cats, err := s.categoryLookup.ListCategories(ctx)
	if err != nil {
		log.Printf("scheduler: listing categories for alarm evaluation: %v", err)
		return out
	}
	for _, c := range cats {
		out[strings.ToLower(c.Name)] = c.ID
	}
	return out
}

func (s *Scheduler) buildProjectNameMap(ctx context.Context) map[string]int64 {
	out := map[string]int64{}
	projs, err := s.projectLookup.ListProjects(ctx, false)
	if err != nil {
		log.Printf("scheduler: listing projects for alarm evaluation: %v", err)
		return out
	}
	for _, p := range projs {
		out[strings.ToLower(p.Name)] = p.ID
	}
	return out
}

// checkActivityAlarm evaluates one activity_based alarm's trigger
// condition against TODAY's accumulated time (local calendar day,
// matching internal/periods' own day-window convention - section 70),
// firing at most once per day.
func (s *Scheduler) checkActivityAlarm(ctx context.Context, a alarms.Alarm, now time.Time, categoryByName, projectByName map[string]int64) {
	cond, err := alarms.ParseTriggerCondition(a.TriggerCondition)
	if err != nil {
		log.Printf("scheduler: alarm %d has an invalid trigger condition %q: %v", a.ID, a.TriggerCondition, err)
		return
	}

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if a.LastFiredAt != nil && !a.LastFiredAt.Before(dayStart) {
		return // already fired today - re-arms tomorrow, per the doc comment above
	}
	dayEnd := dayStart.AddDate(0, 0, 1)

	var totalSecs int64
	switch cond.Scope {
	case alarms.ScopeCategory:
		id, ok := categoryByName[strings.ToLower(cond.Name)]
		if !ok {
			return // unknown category name - nothing to evaluate (the person may have renamed/deleted it)
		}
		sums, err := s.activitySums.SumDurationByCategory(ctx, dayStart.Unix(), dayEnd.Unix())
		if err != nil {
			log.Printf("scheduler: summing category durations for alarm %d: %v", a.ID, err)
			return
		}
		totalSecs = sums[id]
	case alarms.ScopeProject:
		id, ok := projectByName[strings.ToLower(cond.Name)]
		if !ok {
			return
		}
		sums, err := s.activitySums.SumDurationByProject(ctx, dayStart.Unix(), dayEnd.Unix())
		if err != nil {
			log.Printf("scheduler: summing project durations for alarm %d: %v", a.ID, err)
			return
		}
		totalSecs = sums[id]
	default:
		return
	}

	if totalSecs < int64(cond.Minutes)*60 {
		return
	}
	s.notifyAndMark(ctx, a, now, true)
}

// notifyAndMark fires the notification and records that the alarm fired.
// recurring selects MarkFiredRecurring (keeps the alarm enabled - used
// for activity_based alarms, which have no recurrence_rule value for
// alarms.Repository.MarkFired to detect and would otherwise be
// incorrectly treated as one-shot) vs MarkFired (which itself checks
// recurrence_rule to decide whether to disable - correct for
// one_time/relative/schedule vs recurring).
func (s *Scheduler) notifyAndMark(ctx context.Context, a alarms.Alarm, at time.Time, recurring bool) {
	title, body := alarmMessage(a)
	// An alarm the person explicitly marked silent stays silent; every
	// other alarm is heard. This is the only value of Alarm.Sound the
	// scheduler acts on - a named sound is the OS notification's job
	// (see internal/notify.WailsDispatcher.Send), because only the OS
	// knows its own sound library.
	if !strings.EqualFold(a.Sound, "silent") && s.cfg.PlaySound != nil {
		s.cfg.PlaySound()
	}
	if err := s.notifier.Notify(ctx, notify.SourceAlarm, a.ID, title, body, a.Sound); err != nil {
		log.Printf("scheduler: notifying alarm %d: %v", a.ID, err)
		// Still mark fired below - the underlying event (time's up)
		// happened regardless of whether the OS banner rendered
		// (section 56).
	}

	var err error
	if recurring {
		err = s.alarmStore.MarkFiredRecurring(ctx, a.ID, at)
	} else {
		err = s.alarmStore.MarkFired(ctx, a.ID, at)
	}
	if err != nil {
		log.Printf("scheduler: marking alarm %d fired: %v", a.ID, err)
		return
	}
	if s.cfg.OnAlarmFired != nil {
		s.cfg.OnAlarmFired(a)
	}
}

func alarmMessage(a alarms.Alarm) (title, body string) {
	label := a.Label
	if label == "" {
		label = "STELLAREI alarm"
	}
	switch a.AlarmType {
	case alarms.TypeActivityBased:
		return "Time check", label
	case alarms.TypeSchedule:
		return "Scheduled", label
	default:
		return "Alarm", label
	}
}
