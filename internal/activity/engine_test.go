package activity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"stellarei/internal/platform"
	"stellarei/internal/sessions"
)

// --- fakes -------------------------------------------------------------

type fakeWatcher struct {
	info platform.ActiveWindowInfo
	err  error
}

func (f *fakeWatcher) ActiveWindow() (platform.ActiveWindowInfo, error) { return f.info, f.err }

type fakeIdleWatcher struct {
	dur time.Duration
	err error
}

func (f *fakeIdleWatcher) IdleDuration() (time.Duration, error) { return f.dur, f.err }

// fakeClock lets Pause()'s internal "now" be pinned to the same
// fabricated timeline the tests drive tick() with, instead of real
// wall-clock time - see Config.Clock's doc comment.
type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

type insertedEvent struct {
	appID int64
	ev    CompletedEvent
}

type fakeRepo struct {
	apps   map[string]int64
	nextID int64
	events []insertedEvent
}

func newFakeRepo() *fakeRepo { return &fakeRepo{apps: map[string]int64{}} }

func (f *fakeRepo) ResolveApplicationID(_ context.Context, name string) (int64, error) {
	if id, ok := f.apps[name]; ok {
		return id, nil
	}
	f.nextID++
	f.apps[name] = f.nextID
	return f.nextID, nil
}

func (f *fakeRepo) InsertEvent(_ context.Context, appID int64, ev CompletedEvent) (int64, error) {
	f.events = append(f.events, insertedEvent{appID, ev})
	return int64(len(f.events)), nil
}

type fakeSessionRepo struct {
	sessions []sessions.BuiltSession
}

func (f *fakeSessionRepo) InsertSession(_ context.Context, s sessions.BuiltSession) (int64, error) {
	f.sessions = append(f.sessions, s)
	return int64(len(f.sessions)), nil
}

// The Engine only ever calls InsertSession - these exist solely to
// satisfy the broadened sessions.Repository interface (added across
// Phases 3-4 for Timeline/Analytics needs) and are never exercised here.
func (f *fakeSessionRepo) ListForRange(context.Context, int64, int64) ([]sessions.View, error) {
	return nil, nil
}
func (f *fakeSessionRepo) UpdateCorrection(context.Context, int64, int64, int64, bool, string) error {
	return nil
}
func (f *fakeSessionRepo) Split(context.Context, int64, int64) (int64, error) { return 0, nil }
func (f *fakeSessionRepo) Merge(context.Context, int64, int64) error          { return nil }
func (f *fakeSessionRepo) SumDurationByCategory(context.Context, int64, int64) (map[int64]int64, error) {
	return nil, nil
}
func (f *fakeSessionRepo) SumDurationByProject(context.Context, int64, int64) (map[int64]int64, error) {
	return nil, nil
}
func (f *fakeSessionRepo) ContextSwitchStats(context.Context, int64, int64) (sessions.ContextSwitchStats, error) {
	return sessions.ContextSwitchStats{}, nil
}
func (f *fakeSessionRepo) LastActivityByProject(context.Context) (map[int64]int64, error) {
	return nil, nil
}

type fakeProjectResolver struct {
	projectID int64
	err       error
	calls     []struct{ ProcessName, WindowTitle string }
}

func (f *fakeProjectResolver) ResolveProjectID(_ context.Context, processName, windowTitle string) (int64, error) {
	f.calls = append(f.calls, struct{ ProcessName, WindowTitle string }{processName, windowTitle})
	return f.projectID, f.err
}

type fakeCategoryResolver struct {
	categoryID int64
	err        error
	calls      []struct {
		ApplicationID            int64
		ProcessName, WindowTitle string
	}
}

func (f *fakeCategoryResolver) ResolveCategoryID(_ context.Context, applicationID int64, processName, windowTitle string) (int64, error) {
	f.calls = append(f.calls, struct {
		ApplicationID            int64
		ProcessName, WindowTitle string
	}{applicationID, processName, windowTitle})
	return f.categoryID, f.err
}

func newTestEngine(watcher *fakeWatcher, idle *fakeIdleWatcher) (*Engine, *fakeRepo, *fakeSessionRepo) {
	repo := newFakeRepo()
	sessionRepo := &fakeSessionRepo{}
	e := NewEngine(watcher, idle, repo, sessionRepo, Config{
		IdleThreshold: 2 * time.Minute,
		MaxSuspendGap: 60 * time.Second,
	})
	return e, repo, sessionRepo
}

// --- tests ---------------------------------------------------------------

func TestEngine_SameAppAcrossTicksProducesNoWritesUntilSwitch(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe", WindowTitle: "main.go"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	for i := 0; i < 5; i++ {
		e.tick(base.Add(time.Duration(i) * 5 * time.Second))
	}

	if len(repo.events) != 0 {
		t.Fatalf("events persisted mid-run = %d, want 0 (nothing should write until the run ends)", len(repo.events))
	}

	status := e.Status()
	if status.ProcessName != "Code.exe" || status.Paused {
		t.Errorf("Status() = %+v, want in-progress Code.exe, not paused", status)
	}
}

func TestEngine_AppSwitchPersistsEventAndSession(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe", WindowTitle: "main.go"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, sessionRepo := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)
	e.tick(base.Add(30 * time.Second))

	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe", WindowTitle: "GitHub"}
	e.tick(base.Add(60 * time.Second)) // closes the Code.exe *event*

	if len(repo.events) != 1 {
		t.Fatalf("events persisted = %d, want 1", len(repo.events))
	}
	if repo.events[0].ev.ProcessName != "Code.exe" {
		t.Errorf("persisted event process = %q, want Code.exe", repo.events[0].ev.ProcessName)
	}
	if repo.events[0].ev.Duration != 60*time.Second {
		t.Errorf("persisted event duration = %v, want 60s", repo.events[0].ev.Duration)
	}

	// The Code.exe *session* doesn't close yet - the sessionizer, like
	// the debouncer, only knows a run has ended once it sees what comes
	// next. This mirrors real usage: nobody expects "was that a whole
	// session?" to be answerable before the next thing happens.
	if len(sessionRepo.sessions) != 0 {
		t.Fatalf("sessions persisted right after the switch = %d, want 0 (not closed yet)", len(sessionRepo.sessions))
	}

	watcher.info = platform.ActiveWindowInfo{ProcessName: "figma.exe", WindowTitle: "Design"}
	e.tick(base.Add(120 * time.Second)) // NOW the Code.exe session closes

	if len(sessionRepo.sessions) != 1 {
		t.Fatalf("sessions persisted after the second switch = %d, want 1", len(sessionRepo.sessions))
	}
	if sessionRepo.sessions[0].ApplicationID != repo.events[0].appID {
		t.Errorf("session application = %d, want %d (matching the Code.exe event)", sessionRepo.sessions[0].ApplicationID, repo.events[0].appID)
	}
}

func TestEngine_CategoryResolverAssignsCategoryToClosedSession(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	repo := newFakeRepo()
	sessionRepo := &fakeSessionRepo{}
	catResolver := &fakeCategoryResolver{categoryID: 7}

	e := NewEngine(watcher, idle, repo, sessionRepo, Config{
		IdleThreshold:    2 * time.Minute,
		MaxSuspendGap:    60 * time.Second,
		CategoryResolver: catResolver,
	})

	base := at(0)
	e.tick(base)
	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(30 * time.Second))
	watcher.info = platform.ActiveWindowInfo{ProcessName: "figma.exe"}
	e.tick(base.Add(60 * time.Second))

	if len(sessionRepo.sessions) != 1 {
		t.Fatalf("sessions persisted = %d, want 1", len(sessionRepo.sessions))
	}
	if sessionRepo.sessions[0].CategoryID != 7 {
		t.Errorf("session CategoryID = %d, want 7", sessionRepo.sessions[0].CategoryID)
	}
	if len(catResolver.calls) != 1 || catResolver.calls[0].ApplicationID != repo.apps["Code.exe"] {
		t.Errorf("category resolver called with %v, want application id for Code.exe (%d)", catResolver.calls, repo.apps["Code.exe"])
	}
	if catResolver.calls[0].ProcessName != "Code.exe" {
		t.Errorf("category resolver called with process name %q, want %q", catResolver.calls[0].ProcessName, "Code.exe")
	}
}

func TestEngine_ProjectResolverAssignsProjectToClosedSession(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe", WindowTitle: "main.go - Serenity"}}
	idle := &fakeIdleWatcher{dur: 0}
	repo := newFakeRepo()
	sessionRepo := &fakeSessionRepo{}
	resolver := &fakeProjectResolver{projectID: 42}

	e := NewEngine(watcher, idle, repo, sessionRepo, Config{
		IdleThreshold:   2 * time.Minute,
		MaxSuspendGap:   60 * time.Second,
		ProjectResolver: resolver,
	})

	base := at(0)
	e.tick(base)
	e.tick(base.Add(30 * time.Second))

	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(60 * time.Second)) // closes the Code.exe event

	watcher.info = platform.ActiveWindowInfo{ProcessName: "figma.exe"}
	e.tick(base.Add(120 * time.Second)) // NOW the Code.exe session closes

	if len(sessionRepo.sessions) != 1 {
		t.Fatalf("sessions persisted = %d, want 1", len(sessionRepo.sessions))
	}
	if sessionRepo.sessions[0].ProjectID != 42 {
		t.Errorf("session ProjectID = %d, want 42", sessionRepo.sessions[0].ProjectID)
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("resolver called %d times, want 1", len(resolver.calls))
	}
	if resolver.calls[0].ProcessName != "Code.exe" || resolver.calls[0].WindowTitle != "main.go - Serenity" {
		t.Errorf("resolver called with %+v, want Code.exe / \"main.go - Serenity\"", resolver.calls[0])
	}
}

func TestEngine_ProjectResolverErrorLeavesSessionUnassignedWithoutCrashing(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	repo := newFakeRepo()
	sessionRepo := &fakeSessionRepo{}
	resolver := &fakeProjectResolver{err: fmt.Errorf("db unavailable")}

	e := NewEngine(watcher, idle, repo, sessionRepo, Config{
		IdleThreshold:   2 * time.Minute,
		MaxSuspendGap:   60 * time.Second,
		ProjectResolver: resolver,
	})

	base := at(0)
	e.tick(base)
	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(30 * time.Second))
	watcher.info = platform.ActiveWindowInfo{ProcessName: "figma.exe"}
	e.tick(base.Add(60 * time.Second))

	if len(sessionRepo.sessions) != 1 {
		t.Fatalf("sessions persisted = %d, want 1 (a resolver error must not drop the session)", len(sessionRepo.sessions))
	}
	if sessionRepo.sessions[0].ProjectID != 0 {
		t.Errorf("session ProjectID = %d, want 0 (unassigned) when the resolver errors", sessionRepo.sessions[0].ProjectID)
	}
}

func TestEngine_NilProjectResolverLeavesSessionUnassigned(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, _, sessionRepo := newTestEngine(watcher, idle) // no ProjectResolver configured

	base := at(0)
	e.tick(base)
	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(30 * time.Second))
	watcher.info = platform.ActiveWindowInfo{ProcessName: "figma.exe"}
	e.tick(base.Add(60 * time.Second))

	if len(sessionRepo.sessions) != 1 {
		t.Fatalf("sessions persisted = %d, want 1", len(sessionRepo.sessions))
	}
	if sessionRepo.sessions[0].ProjectID != 0 {
		t.Errorf("session ProjectID = %d, want 0 (no resolver configured - matches every Phase 2 test's behavior)", sessionRepo.sessions[0].ProjectID)
	}
}

func TestEngine_PauseFlushesAndStopsRecording(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe", WindowTitle: "main.go"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)
	e.tick(base.Add(30 * time.Second))

	e.Pause(0) // "until resumed"

	if !e.Status().Paused {
		t.Fatal("Status().Paused = false immediately after Pause(0), want true")
	}
	if len(repo.events) != 1 {
		t.Fatalf("events after pause = %d, want 1 (the in-progress run must be flushed on pause)", len(repo.events))
	}

	// Further ticks while paused must not record anything, even though
	// the fake watcher keeps "seeing" Code.exe.
	e.tick(base.Add(60 * time.Second))
	e.tick(base.Add(90 * time.Second))
	if len(repo.events) != 1 {
		t.Fatalf("events while paused = %d, want still 1 (no recording while paused)", len(repo.events))
	}
	if status := e.Status(); status.ProcessName != "" {
		t.Errorf("Status().ProcessName while paused = %q, want empty", status.ProcessName)
	}
}

func TestEngine_ResumeAllowsTrackingAgain(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	e.Pause(0)
	e.Resume()

	if e.Status().Paused {
		t.Fatal("Status().Paused = true after Resume(), want false")
	}

	e.tick(base)
	e.tick(base.Add(10 * time.Second))
	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(20 * time.Second))

	if len(repo.events) != 1 {
		t.Fatalf("events after resume + switch = %d, want 1 (tracking resumed normally)", len(repo.events))
	}
}

func TestEngine_PrivacyModeStopsRecording(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe", WindowTitle: "main.go"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)
	e.tick(base.Add(30 * time.Second))

	e.SetPrivacyMode(true)

	status := e.Status()
	if !status.Paused {
		t.Fatal("Status().Paused = false after SetPrivacyMode(true), want true")
	}
	if !status.PrivacyMode {
		t.Fatal("Status().PrivacyMode = false after SetPrivacyMode(true), want true")
	}
	if len(repo.events) != 1 {
		t.Fatalf("events after enabling privacy mode = %d, want 1 (the in-progress run must be flushed)", len(repo.events))
	}

	e.tick(base.Add(60 * time.Second))
	if len(repo.events) != 1 {
		t.Fatalf("events while in privacy mode = %d, want still 1 (no recording)", len(repo.events))
	}
}

func TestEngine_PrivacyModeDoesNotClearAnUnrelatedManualPause(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, _, _ := newTestEngine(watcher, idle)

	e.Pause(0) // an unrelated manual "away from desk" pause
	e.SetPrivacyMode(true)
	e.SetPrivacyMode(false) // turning privacy mode back off

	// The manual pause from before must still be in effect - turning
	// Privacy Mode off must never silently resume tracking that was
	// paused for a completely different reason.
	if !e.Status().Paused {
		t.Fatal("Status().Paused = false after disabling privacy mode, want true (the earlier manual Pause(0) must still hold)")
	}
}

func TestEngine_ManualResumeDoesNotClearPrivacyMode(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, _, _ := newTestEngine(watcher, idle)

	e.SetPrivacyMode(true)
	e.Resume() // clicking "Resume Tracking" in the tray, unrelated to Privacy Mode

	status := e.Status()
	if !status.PrivacyMode {
		t.Fatal("PrivacyMode = false after Resume(), want true (Resume must only clear Pause, never Privacy Mode)")
	}
	if !status.Paused {
		t.Fatal("Paused = false after Resume() while Privacy Mode is still on, want true")
	}
}

func TestEngine_PrivacyModeOffWithNoOtherPauseResumesTracking(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	e.SetPrivacyMode(true)
	e.SetPrivacyMode(false)

	if e.Status().Paused {
		t.Fatal("Status().Paused = true after disabling privacy mode with no other pause active, want false")
	}

	base := at(100)
	e.tick(base)
	e.tick(base.Add(10 * time.Second))
	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(20 * time.Second))
	if len(repo.events) != 1 {
		t.Fatalf("events after privacy mode off = %d, want 1 (tracking resumed normally)", len(repo.events))
	}
}

func TestEngine_TimedPauseAutoResumes(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idleWatcher := &fakeIdleWatcher{dur: 0}

	fc := &fakeClock{now: at(0)}
	repo := newFakeRepo()
	sessionRepo := &fakeSessionRepo{}
	e := NewEngine(watcher, idleWatcher, repo, sessionRepo, Config{
		IdleThreshold: 2 * time.Minute,
		MaxSuspendGap: 60 * time.Second,
		Clock:         fc.Now,
	})

	e.tick(at(0))
	e.Pause(30 * time.Second) // Pause() reads fc.Now() = at(0); expiry = at(30)

	if !e.Status().Paused {
		t.Fatal("Status().Paused = false right after Pause(30s), want true")
	}

	// A tick just before expiry must still be treated as paused.
	e.tick(at(29))
	if !e.Status().Paused {
		t.Fatal("Status().Paused = false at t=29s (before the 30s pause expires), want true")
	}
	if len(repo.events) != 0 {
		t.Fatalf("events recorded while still paused = %d, want 0", len(repo.events))
	}

	// A tick at/after expiry must auto-resume.
	e.tick(at(31))
	if e.Status().Paused {
		t.Fatal("Status().Paused = true at t=31s (after the 30s pause expired), want false - should have auto-resumed")
	}

	// And tracking should work normally again after auto-resume.
	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(at(40))
	if len(repo.events) != 1 {
		t.Fatalf("events after auto-resume + switch = %d, want 1", len(repo.events))
	}
}

func TestEngine_WatcherErrorSkipsTickWithoutCrashing(t *testing.T) {
	watcher := &fakeWatcher{err: platform.ErrUnavailable}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)
	e.tick(base.Add(5 * time.Second))

	if len(repo.events) != 0 {
		t.Fatalf("events with an unavailable watcher = %d, want 0", len(repo.events))
	}
	// Must not have panicked to get here - that's the actual assertion.
}

func TestEngine_SubSecondRunsAreDropped(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)

	watcher.info = platform.ActiveWindowInfo{ProcessName: "chrome.exe"}
	e.tick(base.Add(400 * time.Millisecond)) // switch after <1s - Code.exe run should be dropped as noise

	watcher.info = platform.ActiveWindowInfo{ProcessName: "Code.exe"}
	e.tick(base.Add(800 * time.Millisecond)) // and switching back drops the chrome.exe run too

	if len(repo.events) != 0 {
		t.Fatalf("events from sub-second runs = %d, want 0 (noise-filtered)", len(repo.events))
	}
}

func TestEngine_IdleTransitionClosesSessionOnceTheIdlePeriodItselfEnds(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, sessionRepo := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)
	e.tick(base.Add(30 * time.Second))

	idle.dur = 10 * time.Minute        // now well past the 2-minute threshold
	e.tick(base.Add(60 * time.Second)) // closes the active Code.exe event

	if len(repo.events) != 1 {
		t.Fatalf("events after idle transition = %d, want 1 (the active Code.exe run)", len(repo.events))
	}
	if repo.events[0].ev.Idle {
		t.Error("persisted event Idle = true, want false - that's the active run that ended, not the idle one")
	}

	// The session doesn't close the moment idling STARTS - only once the
	// idle PERIOD ITSELF ends (same look-ahead-by-one reasoning as the
	// app-switch test above: persistLocked closes a session when it
	// persists an Idle=true event, i.e. when the idle run finishes).
	if len(sessionRepo.sessions) != 0 {
		t.Fatalf("sessions immediately after going idle = %d, want 0", len(sessionRepo.sessions))
	}

	idle.dur = 0 // active again - ends the idle period. Stay within
	// MaxSuspendGap of the last tick so this reads as "idle period
	// ended", not "system slept" (see the suspend-gap test below for
	// that scenario, tested deliberately).
	e.tick(base.Add(100 * time.Second))

	if len(repo.events) != 2 {
		t.Fatalf("events after returning from idle = %d, want 2 (the active run, then the idle run)", len(repo.events))
	}
	if !repo.events[1].ev.Idle {
		t.Error("second persisted event Idle = false, want true (the idle period that just ended)")
	}
	if len(sessionRepo.sessions) != 1 {
		t.Fatalf("sessions after the idle period ends = %d, want 1", len(sessionRepo.sessions))
	}
}

func TestEngine_SuspendGapClosesRunAtLastPollNotAtResumeTime(t *testing.T) {
	watcher := &fakeWatcher{info: platform.ActiveWindowInfo{ProcessName: "Code.exe"}}
	idle := &fakeIdleWatcher{dur: 0}
	e, repo, _ := newTestEngine(watcher, idle)

	base := at(0)
	e.tick(base)
	e.tick(base.Add(10 * time.Second)) // lastPollAt = base+10s

	// Simulate the system sleeping for an hour: the next tick arrives
	// far beyond MaxSuspendGap (60s).
	resumeTime := base.Add(10*time.Second + time.Hour)
	e.tick(resumeTime)

	if len(repo.events) != 1 {
		t.Fatalf("events after suspend gap = %d, want 1 (the pre-sleep run, closed out)", len(repo.events))
	}
	if repo.events[0].ev.Duration != 10*time.Second {
		t.Errorf("pre-sleep run duration = %v, want 10s (closed at last successful poll, not inflated across the sleep gap)", repo.events[0].ev.Duration)
	}
}
