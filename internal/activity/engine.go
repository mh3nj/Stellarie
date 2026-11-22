package activity

import (
	"context"
	"log"
	"sync"
	"time"

	"stellarei/internal/platform"
	"stellarei/internal/sessions"
)

// State is a snapshot of what the Activity Engine is doing right now
// what the tray label, the Overview page, and Focus Mode all read from.
type State struct {
	ProcessName  string
	WindowTitle  string
	Idle         bool
	Paused       bool
	PrivacyMode  bool      // true when Privacy Mode is active - see SetPrivacyMode's doc comment
	RunStartedAt time.Time // zero if nothing is in progress (paused, or no sample yet)
	UpdatedAt    time.Time
}

// CategoryResolver assigns a category to a closing session based on its
// application, and (as of category_rules) its process name and window
// title too (spec sections 10/65 - section 65's rule-engine example
// names categories, not just projects). Optional, same nil-is-fine
// contract as ProjectResolver. Implemented by
// internal/categories.Resolver - kept as an interface here for the same
// import-cycle reason.
type CategoryResolver interface {
	ResolveCategoryID(ctx context.Context, applicationID int64, processName, windowTitle string) (int64, error)
}

// ProjectResolver assigns a project to a closing session based on its
// process name and window title (spec sections 11/65's rule engine).
// Optional - a nil resolver (the default, and everything before Phase 3)
// leaves every session's ProjectID at 0 (unassigned), exactly matching
// Phase 2 behavior before the Project System existed. Implemented by
// internal/projects.Resolver; kept as an interface here so
// internal/activity never needs to import internal/projects (or, worse,
// app/services, which would create an import cycle since app/services
// already imports internal/activity).
type ProjectResolver interface {
	ResolveProjectID(ctx context.Context, processName, windowTitle string) (int64, error)
}

// Config controls Engine behavior. Zero-value fields fall back to sane
// defaults in NewEngine - callers normally only need to set the fields
// they're overriding from settings.
type Config struct {
	// PollInterval is how often the foreground window and idle state are
	// sampled. Section 50 asks for near-zero idle CPU, which a poll every
	// few seconds comfortably delivers - this is not a high-frequency
	// hook, just a periodic check.
	PollInterval time.Duration

	// IdleThreshold is how long without input before a sample counts as
	// idle (section 9). Wired from the idle_threshold_seconds setting.
	IdleThreshold time.Duration

	// MaxSuspendGap is how large a gap between consecutive polls has to
	// be before the Engine assumes the system was asleep/hibernated and
	// closes out the in-progress run at the LAST successful poll time
	// instead of silently attributing the whole sleep duration to
	// whatever was foreground beforehand.
	MaxSuspendGap time.Duration

	// OnStateChange, if set, is called after every tick with the current
	// State - main.go wires this to update the tray label and emit a
	// Wails event for the frontend. Called synchronously from the poll
	// goroutine (and from Pause/Resume), so it must not block.
	OnStateChange func(State)

	// Clock returns the current time. Defaults to time.Now in NewEngine.
	// Overridable so Pause/Resume/Status timing can be tested with a
	// fabricated clock instead of depending on real wall-clock time
	// tick() itself always takes `now` as an explicit argument and
	// doesn't need this, but Pause/Resume/Status have no such argument
	// since the frontend and tray call them with no timestamp to give.
	Clock func() time.Time

	// ProjectResolver assigns a project to each closing session - see
	// the ProjectResolver interface's doc comment. Left nil, sessions
	// simply get no project (ProjectID 0), which is a fully valid state
	// the rest of the app already handles (a session can always be
	// manually assigned a project later - section 63).
	ProjectResolver ProjectResolver

	// CategoryResolver assigns a category to each closing session - see
	// the CategoryResolver interface's doc comment. Same nil-is-valid
	// contract as ProjectResolver.
	CategoryResolver CategoryResolver
}

const (
	defaultPollInterval  = 5 * time.Second
	defaultIdleThreshold = 2 * time.Minute // section 9's stated default
	defaultMaxSuspendGap = 60 * time.Second
)

// Engine is the Activity Engine. All exported methods are safe for
// concurrent use - a single mutex guards every field the poll goroutine
// and frontend-triggered calls (Pause/Resume/Status) both touch.
type Engine struct {
	watcher     platform.ActiveWindowWatcher
	idleWatcher platform.IdleWatcher
	repo        Repository
	sessionRepo sessions.Repository

	cfg Config

	mu            sync.Mutex
	debouncer     *Debouncer
	sessionizer   *sessions.IncrementalSessionizer
	lastPollAt    time.Time
	pausedUntil   *time.Time // nil + !pausedForever = not paused
	pausedForever bool
	privacyMode   bool // set only by SetPrivacyMode, independent of pausedForever/pausedUntil - see that method's doc comment

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewEngine constructs an Engine. Unset Config fields fall back to the
// documented defaults.
func NewEngine(watcher platform.ActiveWindowWatcher, idleWatcher platform.IdleWatcher, repo Repository, sessionRepo sessions.Repository, cfg Config) *Engine {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.IdleThreshold <= 0 {
		cfg.IdleThreshold = defaultIdleThreshold
	}
	if cfg.MaxSuspendGap <= 0 {
		cfg.MaxSuspendGap = defaultMaxSuspendGap
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Engine{
		watcher:     watcher,
		idleWatcher: idleWatcher,
		repo:        repo,
		sessionRepo: sessionRepo,
		cfg:         cfg,
		debouncer:   NewDebouncer(),
		sessionizer: sessions.NewIncrementalSessionizer(),
	}
}

// Start launches the poll loop in a background goroutine and returns
// immediately. ctx's cancellation (or a call to Stop) ends the loop.
func (e *Engine) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.wg.Add(1)
	go e.run(runCtx)
}

// Stop cancels the poll loop and blocks until it has flushed any
// in-progress run and exited - no goroutine leak, per section 51.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

func (e *Engine) run(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.shutdown()
			return
		case now := <-ticker.C:
			e.tick(now)
		}
	}
}

func (e *Engine) shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Bounded, not context.Background()-forever: shutdown must not hang
	// indefinitely if the database is somehow stuck.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e.flushLocked(ctx, e.cfg.Clock())
}

// tick is the entire per-poll decision, called from the real ticker loop
// and directly from tests with a fabricated `now`.
func (e *Engine) tick(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.maybeAutoResumeLocked(now)

	if e.pausedForever || e.pausedUntil != nil || e.privacyMode {
		e.lastPollAt = now
		e.notifyLocked(now)
		return
	}

	// A gap much larger than the poll interval means the system likely
	// slept/hibernated between ticks. Close out whatever was running at
	// the LAST successful poll rather than let the sleep duration get
	// silently attributed to it.
	if !e.lastPollAt.IsZero() && now.Sub(e.lastPollAt) > e.cfg.MaxSuspendGap {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e.flushLocked(ctx, e.lastPollAt)
		cancel()
	}

	info, err := e.watcher.ActiveWindow()
	if err != nil {
		// No usable sample this tick (transient failure, no focused
		// window, etc.) - section 56: a subsystem hiccup must not stop
		// tracking overall. lastPollAt still advances so the suspend-gap
		// check above stays accurate on the next successful tick.
		e.lastPollAt = now
		e.notifyLocked(now)
		return
	}

	idleDur, err := e.idleWatcher.IdleDuration()
	if err != nil {
		// Unknown idle state defaults to "active" - see the package doc
		// for the reasoning: we'd rather under-report idle time than
		// wrongly mark someone idle when we simply couldn't ask.
		idleDur = 0
	}
	idle := idleDur >= e.cfg.IdleThreshold

	sample := Sample{
		Time:        now,
		ProcessName: info.ProcessName,
		WindowTitle: info.WindowTitle,
		Idle:        idle,
	}

	if completed := e.debouncer.Feed(sample); completed != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e.persistLocked(ctx, *completed)
		cancel()
	}

	e.lastPollAt = now
	e.notifyLocked(now)
}

// maybeAutoResumeLocked clears an expired timed pause. Must be called
// with e.mu held.
func (e *Engine) maybeAutoResumeLocked(now time.Time) {
	if e.pausedUntil != nil && !now.Before(*e.pausedUntil) {
		e.pausedUntil = nil
	}
}

// flushLocked closes out whatever run is in progress as of `at` and
// persists it (and any session it closes). Must be called with e.mu held.
func (e *Engine) flushLocked(ctx context.Context, at time.Time) {
	if completed := e.debouncer.Flush(at); completed != nil {
		e.persistLocked(ctx, *completed)
	}
}

// persistLocked writes one completed event and feeds it through the
// sessionizer, persisting a session too if one just closed. Errors are
// logged, not returned - the poll loop has no caller to propagate them
// to, and one failed write must not stop the next tick from trying
// again (section 56).
func (e *Engine) persistLocked(ctx context.Context, completed CompletedEvent) {
	// Sub-second runs are poll noise (a flicker during alt-tab, a
	// window manager animation frame) rather than meaningful activity
	// dropped here rather than persisted, per section 7's spirit of not
	// storing an event per tick. This is the only place raw activity
	// data is intentionally discarded, and it's a fixed, documented
	// threshold, not a judgment call about the activity's importance.
	if completed.Duration < time.Second {
		return
	}

	appID, err := e.repo.ResolveApplicationID(ctx, completed.ProcessName)
	if err != nil {
		log.Printf("activity: resolving application: %v", err)
		return
	}

	eventID, err := e.repo.InsertEvent(ctx, appID, completed)
	if err != nil {
		log.Printf("activity: inserting event: %v", err)
		return
	}

	if completed.Idle {
		// Idle events participate in InsertEvent (so they're queryable
		// as "when was I away") but never enter the sessionizer - idle
		// time is never part of a session (section 9).
		if closed := e.sessionizer.Flush(); closed != nil {
			e.resolveAndInsertSession(ctx, closed)
		}
		return
	}

	raw := sessions.RawEvent{
		ID:            eventID,
		StartedAt:     completed.StartedAt,
		Duration:      completed.Duration,
		ApplicationID: appID,
		ProcessName:   completed.ProcessName,
		WindowTitle:   completed.WindowTitle,
		Idle:          false,
		Confidence:    1.0, // Phase 2 has no classification uncertainty yet - see sessions doc comment
	}
	if closed := e.sessionizer.Feed(raw); closed != nil {
		e.resolveAndInsertSession(ctx, closed)
	}
}

// resolveAndInsertSession assigns a project (if a resolver is
// configured) and persists the session. Split out from persistLocked
// since both the idle-close and app-switch-close paths need it.
func (e *Engine) resolveAndInsertSession(ctx context.Context, closed *sessions.BuiltSession) {
	if e.cfg.ProjectResolver != nil {
		id, err := e.cfg.ProjectResolver.ResolveProjectID(ctx, closed.ProcessName, closed.WindowTitle)
		if err != nil {
			log.Printf("activity: resolving project for session: %v", err)
		} else {
			closed.ProjectID = id
		}
	}
	if e.cfg.CategoryResolver != nil {
		id, err := e.cfg.CategoryResolver.ResolveCategoryID(ctx, closed.ApplicationID, closed.ProcessName, closed.WindowTitle)
		if err != nil {
			log.Printf("activity: resolving category for session: %v", err)
		} else {
			closed.CategoryID = id
		}
	}
	if _, err := e.sessionRepo.InsertSession(ctx, *closed); err != nil {
		log.Printf("activity: inserting session: %v", err)
	}
}

// notifyLocked calls the configured OnStateChange callback, if any, with
// the current state. Must be called with e.mu held - the callback itself
// must not call back into the Engine or it will deadlock.
func (e *Engine) notifyLocked(now time.Time) {
	if e.cfg.OnStateChange == nil {
		return
	}

	state := State{Paused: e.pausedForever || e.pausedUntil != nil || e.privacyMode, PrivacyMode: e.privacyMode, UpdatedAt: now}
	if !state.Paused {
		if s, ok := e.debouncer.InProgress(); ok {
			state.ProcessName = s.ProcessName
			state.WindowTitle = s.WindowTitle
			state.Idle = s.Idle
			state.RunStartedAt = s.Time
		}
	}
	e.cfg.OnStateChange(state)
}

// SetOnStateChange installs (or replaces) the state-change callback.
// Exists separately from Config because the callback commonly needs to
// reference things - like the tray, in main.go - that can't be built
// until after the Engine itself exists (the tray's Pause/Resume menu
// items need the Engine reference), so main.go builds the Engine first,
// builds the tray with it, and only then wires this callback in. Safe to
// call before or after Start.
func (e *Engine) SetOnStateChange(fn func(State)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.OnStateChange = fn
}

// used by the Wails-bound ActivityService.GetStatus.
func (e *Engine) Status() State {
	e.mu.Lock()
	defer e.mu.Unlock()

	state := State{Paused: e.pausedForever || e.pausedUntil != nil || e.privacyMode, PrivacyMode: e.privacyMode, UpdatedAt: e.cfg.Clock()}
	if !state.Paused {
		if s, ok := e.debouncer.InProgress(); ok {
			state.ProcessName = s.ProcessName
			state.WindowTitle = s.WindowTitle
			state.Idle = s.Idle
			state.RunStartedAt = s.Time
		}
	}
	return state
}

// Pause stops tracking. duration <= 0 means "until resumed" (section 25);
// otherwise tracking auto-resumes after duration elapses. Any run
// currently in progress is flushed and persisted immediately - pausing
// must not silently extend whatever was being tracked at the moment of
// the click.
func (e *Engine) Pause(duration time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.cfg.Clock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	e.flushLocked(ctx, now)
	cancel()

	if duration <= 0 {
		e.pausedForever = true
		e.pausedUntil = nil
	} else {
		until := now.Add(duration)
		e.pausedUntil = &until
		e.pausedForever = false
	}
	e.notifyLocked(now)
}

// Resume clears any pause, timed or indefinite. Per section 25, resuming
// is always an explicit, visible action - this method is that action;
// nothing in the Engine resumes tracking silently except a timed pause's
// own expiry, which the UI already showed a countdown for. Resume
// deliberately does NOT clear Privacy Mode - see SetPrivacyMode.
func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pausedForever = false
	e.pausedUntil = nil
	e.notifyLocked(e.cfg.Clock())
}

// SetPrivacyMode turns Privacy Mode on or off (spec section 26). Kept as
// a separate flag from Pause/Resume above rather than reusing
// pausedForever, even though both stop tracking the same way: a person
// might have manually paused for an unrelated reason (away from their
// desk) and separately want Privacy Mode for a specific sensitive task,
// or vice versa - one turning off must never silently resume the other.
// Tracking is stopped whenever EITHER is active (see tick/notifyLocked/
// Status's own `|| e.privacyMode` checks); turning Privacy Mode off only
// resumes tracking if nothing else is also holding it paused.
func (e *Engine) SetPrivacyMode(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.cfg.Clock()
	if enabled && !e.privacyMode {
		// Same reasoning as Pause: flush whatever's in progress right
		// now rather than letting Privacy Mode silently extend it.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e.flushLocked(ctx, now)
		cancel()
	}
	e.privacyMode = enabled
	e.notifyLocked(now)
}
