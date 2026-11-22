// STELLAREI - Personal Time Intelligence System
//
// This is the entrypoint through Phase 10: native application shell
// (single instance, tray, startup registration, close behavior,
// settings), the Activity Engine (process/window/idle detection,
// debounced event storage, sessionization), Timeline, Analytics
// (categories/projects/budgets/goals), Timers + Alarms + Notifications,
// Focus Mode, "I HAVE NO IDEA" (entirely AI-free suggestion generation
// see internal/idea's package doc), the Privacy Center (Privacy Mode,
// Screenshot Memory, per-application privacy - see internal/privacy's
// package doc, including its honest scope note on browser tracking),
// and Backup/Import/Export (see internal/backup's package doc for the
// restart-to-apply restore design and why it's the right call, not a
// shortcut). Phase 8 (Theme Studio) was also brought forward ahead of
// schedule. Everything else in the full spec is a later phase - see
// internal/tray/tray.go's TODO block and README.md for the exact
// roadmap. Nothing below pretends a later-phase feature already works.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"stellarei/app/services"
	"stellarei/internal/activity"
	"stellarei/internal/alert"
	"stellarei/internal/alarms"
	"stellarei/internal/backup"
	"stellarei/internal/budgets"
	"stellarei/internal/categories"
	"stellarei/internal/closebehavior"
	"stellarei/internal/config"
	"stellarei/internal/database"
	"stellarei/internal/goals"
	"stellarei/internal/idea"
	"stellarei/internal/notify"
	"stellarei/internal/patterns"
	"stellarei/internal/platform"
	"stellarei/internal/privacy"
	"stellarei/internal/projects"
	"stellarei/internal/scheduler"
	"stellarei/internal/sessions"
	"stellarei/internal/shortcuts"
	"stellarei/internal/theme"
	"stellarei/internal/timers"
	"stellarei/internal/tray"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// logFanout writes each log line to every target, ignoring a target that
// fails. io.MultiWriter is the obvious choice and is the wrong one here:
// it stops at the first writer that errors, and in the shipped build the
// FIRST writer that errors is os.Stderr - a GUI-subsystem binary (PE
// subsystem 2) has no console, so its stderr handle is invalid and every
// write to it fails. With stderr first in an io.MultiWriter the log file
// stayed forever empty, which is exactly the situation this logging
// exists to make diagnosable.
type logFanout struct{ targets []io.Writer }

func (l logFanout) Write(p []byte) (int, error) {
	for _, target := range l.targets {
		_, _ = target.Write(p)
	}
	return len(p), nil
}

// applyPendingRestore checks for a restore staged by
// services.BackupService.RestoreBackup and, if one exists, swaps it into
// place - see internal/backup's own doc comment for why this specific
// moment (before the database is opened, at the very start of a launch)
// is the only safe time to do it. A no-op when nothing is pending, which
// is the overwhelmingly common case on every ordinary startup.
func applyPendingRestore(markerPath string, paths *config.Paths) error {
	marker, err := backup.ReadRestoreMarker(markerPath)
	if err != nil {
		return fmt.Errorf("reading restore marker: %w", err)
	}
	if marker == nil {
		return nil
	}

	if _, err := os.Stat(marker.StagingDBPath); err != nil {
		return fmt.Errorf("staged database %s is missing: %w", marker.StagingDBPath, err)
	}

	// Remove the live database and its WAL/SHM sidecar files before
	// copying the staged one into place - leftover -wal/-shm files from
	// the PREVIOUS database would otherwise be replayed against the
	// newly-restored one on next open, silently reintroducing exactly
	// the data the restore was meant to remove.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(paths.Database + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing existing %s%s: %w", paths.Database, suffix, err)
		}
	}
	if err := copyRestoreFile(marker.StagingDBPath, paths.Database); err != nil {
		return fmt.Errorf("copying restored database into place: %w", err)
	}

	if marker.StagingScreenshotsDir != "" {
		if err := os.RemoveAll(paths.Screenshots); err != nil {
			return fmt.Errorf("clearing existing screenshots: %w", err)
		}
		if err := os.Rename(marker.StagingScreenshotsDir, paths.Screenshots); err != nil {
			return fmt.Errorf("restoring screenshots: %w", err)
		}
	}

	if err := os.RemoveAll(filepath.Dir(marker.StagingDBPath)); err != nil {
		log.Printf("stellarei: cleaning up restore staging directory: %v", err)
		// Non-fatal - the restore itself already succeeded; a leftover
		// staging directory is just disk space, not a correctness issue.
	}

	return backup.ClearRestoreMarker(markerPath)
}

func copyRestoreFile(srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// activityStatusAdapter bridges *activity.Engine.Status() to
// privacy.ActivityStatusProvider for the Screenshot Memory Sweeper.
// activity.State only carries a process name, not a resolved
// application id, so this resolves one via the exact same
// ResolveApplicationID upsert-on-lookup the Activity Engine itself
// already calls on every debounced sample - one path turns "a process
// name was seen" into an applications row, not two.
type activityStatusAdapter struct {
	engine *activity.Engine
	repo   activity.Repository
}

func (a *activityStatusAdapter) Status() privacy.ActivityStatus {
	s := a.engine.Status()
	out := privacy.ActivityStatus{ProcessName: s.ProcessName, WindowTitle: s.WindowTitle, Idle: s.Idle, Paused: s.Paused}
	if s.ProcessName != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if id, err := a.repo.ResolveApplicationID(ctx, s.ProcessName); err == nil {
			out.ApplicationID = id
		}
	}
	return out
}

// assets embeds the built frontend (frontend/dist, produced by `npm run
// build` inside frontend/ - see frontend/README.md). "all:" is required
// because Vite's output includes files starting with "_" or ".", which
// go:embed ignores by default without that prefix.
//
//go:embed all:frontend/dist
var assets embed.FS

// Tray icon variants (section 23) - switched based on the ACTIVE
// STELLAREI THEME's computed mode, not the OS's own dark-mode setting.
// See internal/tray.Tray.SetIconForMode's doc comment for why.
//
//go:embed assets/tray-icon-dark.png
var trayIconDark []byte

//go:embed assets/tray-icon-light.png
var trayIconLight []byte

// singleInstanceID must never change across releases - Wails uses it to
// derive the Windows mutex name / macOS lock / Linux dbus name that
// prevents duplicate STELLAREI processes (section 76). Changing it would
// let old and new versions run simultaneously against the same database.
const singleInstanceID = "com.stellarei.desktop"

func main() {
	processStarted := time.Now() // captured first, for Diagnostics' real process-uptime figure - see diagnostics_service.go

	startHidden := flag.Bool("start-hidden", false, "start with the main window hidden (used for run-at-login launches configured for tray/silent startup)")
	flag.Parse()

	paths, err := config.Load()
	if err != nil {
		log.Fatalf("stellarei: resolving app data paths: %v", err)
	}

	// Send the log to a file as well as stderr. The shipped Windows build
	// is a GUI-subsystem binary (PE subsystem 2), so it has no console at
	// all: every log.Printf from the Activity Engine, the Scheduler, the
	// Privacy Sweeper and the notification/audio backends was previously
	// discarded. That made a failure to show or sound an alarm impossible
	// to diagnose - the person saw nothing and there was nothing to read.
	// Logging is best effort: a failure to open the file must not stop
	// STELLAREI from starting, so stderr remains the fallback.
	logFile, err := os.OpenFile(filepath.Join(paths.Logs, "stellarei.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("stellarei: opening log file (continuing to stderr only): %v", err)
	} else {
		log.SetOutput(logFanout{targets: []io.Writer{logFile, os.Stderr}})
		log.Printf("stellarei: log file open at %s", logFile.Name())
	}

	// Apply a staged Restore (spec section 48), if one is pending, BEFORE
	// the live database is opened by anything below - see
	// internal/backup's own doc comment on why the database swap happens
	// here, at the very start of a launch, rather than while STELLAREI
	// is already running. A failure here is loud (log.Fatalf, not a
	// silent skip): if the staged files are missing or corrupted, the
	// person needs to know their restore didn't apply rather than
	// unknowingly keep using the pre-restore database.
	restoreMarkerPath := filepath.Join(paths.Root, "restore-pending.json")
	if err := applyPendingRestore(restoreMarkerPath, paths); err != nil {
		log.Fatalf("stellarei: applying pending restore: %v", err)
	}

	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: paths.Database})
	if err != nil {
		log.Fatalf("stellarei: opening database: %v", err)
	}

	settingsService := services.NewSettingsService(db)
	startupService := services.NewStartupService(settingsService)

	// Optional Local AI (Phase 11, spec section 37 / section 71's
	// Settings > AI): constructed early, right alongside settingsService,
	// because it owns its own slice of the same `settings` table and
	// every later service that wants to call Enhance (IdeaService below,
	// eventually AnalyticsService for Daily Review commentary) needs a
	// reference to its RawService(). Off by default until the person
	// visits Settings > AI - see internal/ai's package doc for the full
	// disabled-by-default / opt-in-per-scope / graceful-degradation
	// contract every one of those later call sites relies on.
	aiService := services.NewAIService(db)

	// Resolve whether the window should start hidden: an explicit
	// --start-hidden flag (passed by our own Run-key/LaunchAgent/autostart
	// entry, see internal/platform) always wins; otherwise fall back to
	// the user's persisted "Startup behavior" setting (section 24).
	hideOnLaunch := *startHidden
	if !hideOnLaunch {
		raw, err := settingsService.GetString(services.KeyStartupBehavior)
		if err != nil {
			log.Printf("stellarei: reading startup behavior: %v", err)
		}
		hideOnLaunch = raw == "tray" || raw == "silent"
	}

	windowService := services.NewWindowService()

	// Project System (Phase 3): the resolver evaluates project_rules
	// against each closing session's process/window-title, assigning a
	// project automatically (spec sections 11/65). Built before the
	// Engine since it's passed in at construction time.
	projectRepo := projects.NewRepository(db)
	projectResolver := projects.NewResolver(projectRepo)
	projectService := services.NewProjectService(projectRepo)

	sessionRepo := sessions.NewRepository(db)
	sessionService := services.NewSessionService(sessionRepo)

	// Category assignment (Phase 4, section 10): resolves a session's
	// category from its application, same nil-is-fine pattern as the
	// project resolver above.
	categoryRepo := categories.NewRepository(db)
	categoryResolver := categories.NewResolver(categoryRepo)
	categoryService := services.NewCategoryService(categoryRepo)

	budgetService := services.NewBudgetService(budgets.NewRepository(db), sessionRepo)
	goalService := services.NewGoalService(goals.NewRepository(db), sessionRepo)
	analyticsService := services.NewAnalyticsService(sessionRepo)

	// Daily Time Receipt (spec section 60): a real, deterministic feature
	// built on top of the three services just above it, with an OPTIONAL
	// AI commentary line (aiService.RawService(), same singleton
	// IdeaService uses) gated on the "daily_reviews" scope - see
	// receipt_service.go's own doc comments for the full contract.
	receiptService := services.NewTimeReceiptService(sessionRepo, goalService, budgetService, aiService.RawService())

	// Pattern Discovery (spec section 31): a real, deterministic feature
	// built from scratch this session (see internal/patterns' package
	// doc) - like receiptService above, this wasn't an AI task at its
	// core. aiService.RawService() wires in the same OPTIONAL, additive-
	// only AI summary layer as everywhere else.
	patternService := services.NewPatternService(sessionRepo, patterns.NewRepository(db), aiService.RawService())

	// Digital Waste Detection (spec section 33) - the one place
	// ScopeSuggestions is actually used; see waste_service.go's own
	// doc comment for why it's distinct from Pattern Discovery's
	// aiNote rather than redundant with it.
	wasteService := services.NewWasteService(sessionRepo, categoryRepo, aiService.RawService())

	// Calendar (spec section 30): a real month-view day-by-day category
	// breakdown, correctly splitting any session that crosses local
	// midnight - see internal/calendar's package doc. No AI involved;
	// not every feature needs the aiNote treatment, and a calendar grid
	// isn't naturally suited to a one-sentence commentary the way a
	// review or suggestion screen is.
	calendarService := services.NewCalendarService(sessionRepo)

	// Search (spec sections 29/62): a real, bounded-vocabulary query
	// parser (internal/search) composed with real project/category/
	// application/session data - see that package's own honest-scope
	// doc comment for exactly what it does and doesn't understand.
	searchService := services.NewSearchService(sessionRepo, projectRepo, categoryRepo)

	// MY YEAR (spec section 61): reuses internal/calendar (busiest
	// month, via internal/myyear's roll-up) and GoalService's existing
	// status math - see myyear_service.go's own doc comments. AINote
	// gated on ScopeSummaries - the scope that had been defined since
	// Phase 11's foundation but never actually wired to a caller until
	// now (an honest correction; see README's Phase 11 section).
	myYearService := services.NewMyYearService(sessionRepo, projectRepo, goalService, aiService.RawService())
	// Timers + alarms + notifications (Phase 5, spec sections 21/22).
	// timerRepo/alarmRepo/notifyRepo are shared between the frontend-facing
	// services below (thin CRUD/read adapters) and the Scheduler further
	// down (the background component that actually notices a countdown
	// hit zero or an alarm came due) - same "one repository, multiple
	// consumers" shape sessionRepo already has with the Activity Engine
	// vs. BudgetService/GoalService/AnalyticsService above.
	timerRepo := timers.NewRepository(db)
	timerService := services.NewTimerService(timerRepo)

	alarmRepo := alarms.NewRepository(db)
	alarmService := services.NewAlarmService(alarmRepo)

	notifyRepo := notify.NewRepository(db)
	notificationService := services.NewNotificationService(notifyRepo)

	// Native OS notification delivery: Wails v3's own notifications
	// service (github.com/wailsapp/wails/v3/pkg/services/notifications),
	// registered as an application.Service like everything else so its
	// platform-native lifecycle hooks run, and wrapped by
	// internal/notify.WailsDispatcher so the Scheduler depends on the
	// small Dispatcher interface rather than Wails directly (section 56
	// if this ever fails to construct cleanly on a given platform, the
	// rest of STELLAREI must keep working; NewNotifier's own nil-check
	// falls back to a no-op dispatcher rather than crashing startup).
	nativeNotifications := notifications.New()
	notifier := notify.NewNotifier(notifyRepo, notify.NewWailsDispatcher(nativeNotifications))
	if _, err := nativeNotifications.RequestNotificationAuthorization(); err != nil {
		// Non-fatal: on Windows/Linux this is a no-op; on macOS a denied
		// authorization just means banners won't show - the in-app
		// notification history (NotificationService) still works, and
		// alarms/timers still fire and get recorded either way.
		log.Printf("stellarei: requesting notification authorization: %v", err)
	}

	// Activity Engine (Phase 2). idle_threshold_seconds falls back to
	// 120s (section 9's stated default) if unset or unparseable.
	idleThresholdSec, err := settingsService.GetInt(services.KeyIdleThresholdSec, 120)
	if err != nil {
		log.Printf("stellarei: reading idle threshold: %v", err)
	}
	activityRepo := activity.NewRepository(db)
	engine := activity.NewEngine(
		platform.NewActiveWindowWatcher(),
		platform.NewIdleWatcher(),
		activityRepo,
		sessionRepo,
		activity.Config{
			IdleThreshold:    time.Duration(idleThresholdSec) * time.Second,
			ProjectResolver:  projectResolver,
			CategoryResolver: categoryResolver,
		},
	)
	activityService := services.NewActivityService(engine)

	// Diagnostics (spec sections 55/80) - see diagnostics_service.go's
	// own doc comment for exactly which parts of those two spec
	// sections are covered vs. genuinely still missing (tray status
	// investigated for real, no such API exists in Wails v3 today).
	diagnosticsService := services.NewDiagnosticsService(db, paths.Database, activityService, startupService, processStarted)

	// Focus Mode (Phase 6, spec section 20): a focus session is a
	// timers.TypeFocus row (see internal/focus's package doc) plus
	// live quality stats computed from real session data - needs
	// `engine` for the live "is the current foreground app blocked"
	// check, so it's constructed here rather than alongside
	// TimerService/AlarmService above.
	focusService := services.NewFocusService(timerRepo, sessionRepo, engine)

	// "I HAVE NO IDEA" (Phase 7, spec sections 34-36): entirely
	// deterministic, AI-free suggestion generation over real data - see
	// internal/idea's own package doc for why. goalService is passed in
	// (both are in package `services`) so IdeaService can reuse its
	// exact goal-progress math rather than recomputing it. aiService.
	// RawService() wires in Phase 11's OPTIONAL commentary layer (see
	// idea_service.go's AINote doc comments) - the three suggestions
	// themselves never depend on it.
	ideaService := services.NewIdeaService(idea.NewRepository(db), projectRepo, goalService, sessionRepo, categoryRepo, aiService.RawService())

	// Privacy Center (Phase 9, spec sections 26-28): Privacy Mode,
	// per-application privacy toggles, exclusions, and Screenshot
	// Memory. privacyRepo is shared between privacyService (the
	// frontend-facing CRUD) and the Sweeper below (the background
	// component that actually captures/purges), same "one repository,
	// multiple consumers" shape sessionRepo already has.
	privacyRepo := privacy.NewRepository(db)
	privacyService := services.NewPrivacyService(privacyRepo, engine, db)

	// Restore Privacy Mode across restarts (spec section 26 - the
	// "PRIVACY MODE ACTIVE" state should still hold if the person
	// reopens STELLAREI while it was on) by re-applying whatever was
	// last persisted; false (off) if it was never set.
	if wasPrivate, err := settingsService.GetBool(services.KeyPrivacyModeEnabled); err != nil {
		log.Printf("stellarei: reading persisted privacy mode: %v", err)
	} else if wasPrivate {
		engine.SetPrivacyMode(true)
	}

	// Backup / Import / Export (Phase 10, spec sections 48-49).
	// restartToApplyRestore relaunches the current executable with the
	// same arguments and quits this process - the only way to safely
	// finish a staged Restore (see internal/backup's own doc comment).
	// A relaunch failure returns an error instead of quitting anyway,
	// so a person is never left with STELLAREI closed and no new
	// process having actually started. appRef is declared here (nil for
	// now) and assigned once `app := application.New(...)` exists further
	// down - this closure is only ever CALLED much later, in response to
	// a person clicking Restart, by which point appRef is long since set;
	// it just can't be written as `app.Quit()` directly yet because `app`
	// itself doesn't exist at this line.
	var appRef *application.App
	restartToApplyRestore := func() error {
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("stellarei: locating executable for restart: %w", err)
		}
		cmd := exec.Command(execPath, os.Args[1:]...)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("stellarei: relaunching: %w", err)
		}
		appRef.Quit()
		return nil
	}
	backupService := services.NewBackupService(
		db, projectRepo, sessionRepo, goalService, budgetService,
		paths.Backups, paths.Exports, paths.Screenshots, paths.Database, restoreMarkerPath,
		restartToApplyRestore,
	)

	// Theme Studio backing service. Computes the initial tray icon mode
	// now, before the tray exists, from whichever theme is currently
	// active - falls back to Dark (Midnight is the shipped default) if
	// anything about that lookup fails, since a wrong tray icon is a far
	// smaller problem than main() failing to start over it.
	themeService := services.NewThemeService(db, settingsService)
	initialThemeMode := theme.Dark
	if activeName, err := themeService.GetActiveThemeName(); err == nil {
		if tokensJSON, err := themeService.GetTokensJSON(activeName); err == nil {
			var parsed struct {
				Background string `json:"background"`
			}
			if json.Unmarshal([]byte(tokensJSON), &parsed) == nil && parsed.Background != "" {
				if mode, err := theme.ComputeMode(parsed.Background); err == nil {
					initialThemeMode = mode
				}
			}
		}
	}

	// secondInstanceWindow is filled in immediately below, once the main
	// window exists; OnSecondInstanceLaunch can only be registered here,
	// before that window is created, so it goes through a small indirection
	// instead of a package-level variable (section 82: no global mutable
	// state).
	var secondInstanceWindow *application.WebviewWindow

	// Global keyboard shortcuts (spec section 45) - action closures and
	// the live registrar are constructed HERE, before `app` exists, via
	// the same lazy-indirection trick restartToApplyRestore uses above:
	// every closure only DEREFERENCES appRef/secondInstanceWindow when it
	// is actually invoked (a shortcut press, or a frontend-triggered
	// Rebind), by which point both are long since assigned. Constructing
	// them here is required because shortcutsService must be in the
	// Services list passed to application.New below - defining it after
	// `app` (as was previously attempted) is a compile error
	// (undefined: shortcutsService at the Services literal).
	actionFuncs := map[shortcuts.Action]func(){
		shortcuts.ActionStartFocus: func() {
			if secondInstanceWindow == nil || appRef == nil {
				return
			}
			secondInstanceWindow.Show()
			secondInstanceWindow.Focus()
			appRef.Event.Emit("tray:open-focus", nil)
		},
		shortcuts.ActionPauseTracking: func() {
			status := activityService.GetStatus()
			if status.Paused {
				engine.Resume()
			} else {
				engine.Pause(0) // "until resumed" - a global shortcut has no UI to pick a duration
			}
		},
		shortcuts.ActionToggleTimer: func() {
			toggleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			active, err := timerRepo.ListActive(toggleCtx)
			if err != nil {
				log.Printf("global shortcut: listing active timers: %v", err)
				return
			}
			if len(active) > 0 {
				if err := timerRepo.Complete(toggleCtx, active[0].ID, time.Now()); err != nil {
					log.Printf("global shortcut: stopping timer %d: %v", active[0].ID, err)
				}
				return
			}
			if _, err := timerRepo.Create(toggleCtx, timers.TypeCountdown, 0, "Quick timer", 25*60, "", time.Now()); err != nil {
				log.Printf("global shortcut: starting quick timer: %v", err)
			}
		},
		shortcuts.ActionIHaveNoIdea: func() {
			if secondInstanceWindow == nil || appRef == nil {
				return
			}
			secondInstanceWindow.Show()
			secondInstanceWindow.Focus()
			appRef.Event.Emit("tray:open-no-idea", nil)
		},
	}

	// registeredAccelerators tracks what's actually live right now, so
	// Rebind knows what to Unregister before Register-ing the
	// replacement. Closes over appRef (not `app`): nil until `app` is
	// assigned just below, non-nil for every real call (initial
	// registration after app creation, or a later frontend-triggered
	// SetShortcut/ResetShortcut).
	registeredAccelerators := make(map[shortcuts.Action]string, len(shortcuts.Actions))
	shortcutRegistrar := services.ShortcutRegistrarFunc(func(action shortcuts.Action, newAccelerator string) error {
		fn, ok := actionFuncs[action]
		if !ok {
			return fmt.Errorf("stellarei: no handler for shortcut action %q", action)
		}
		if appRef == nil {
			return fmt.Errorf("stellarei: shortcut registrar used before app initialization for %q", action)
		}
		if old := registeredAccelerators[action]; old != "" {
			if err := appRef.GlobalShortcut.Unregister(old); err != nil {
				log.Printf("global shortcut: unregistering old accelerator %q for %s (continuing anyway): %v", old, action, err)
			}
		}
		if err := appRef.GlobalShortcut.Register(newAccelerator, fn); err != nil {
			return err
		}
		registeredAccelerators[action] = newAccelerator
		return nil
	})
	shortcutsService := services.NewShortcutsService(settingsService, shortcutRegistrar)

	app := application.New(application.Options{
		Name:        "STELLAREI",
		Description: "A private, local-first time intelligence system.",
		Services: []application.Service{
			application.NewService(settingsService),
			application.NewService(aiService),
			application.NewService(windowService),
			application.NewService(startupService),
			application.NewService(activityService),
			application.NewService(themeService),
			application.NewService(projectService),
			application.NewService(sessionService),
			application.NewService(categoryService),
			application.NewService(budgetService),
			application.NewService(goalService),
			application.NewService(receiptService),
			application.NewService(patternService),
			application.NewService(wasteService),
			application.NewService(calendarService),
			application.NewService(searchService),
			application.NewService(myYearService),
			application.NewService(diagnosticsService),
			application.NewService(analyticsService),
			application.NewService(shortcutsService),
			application.NewService(timerService),
			application.NewService(alarmService),
			application.NewService(notificationService),
			application.NewService(nativeNotifications),
			application.NewService(focusService),
			application.NewService(ideaService),
			application.NewService(privacyService),
			application.NewService(backupService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				// Section 75: a second launch activates the existing
				// instance instead of starting a duplicate tracker.
				if secondInstanceWindow != nil {
					secondInstanceWindow.Show()
					secondInstanceWindow.Focus()
				}
			},
		},
	})
	appRef = app // see restartToApplyRestore's own comment above on why this two-step assignment exists

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "STELLAREI",
		Width:  1280,
		Height: 800,
		Hidden: hideOnLaunch,
	})
	secondInstanceWindow = mainWindow
	windowService.Bind(app, mainWindow)

	// Section 73: close behavior. The window's native close button never
	// gets to decide on its own - it always routes through the persisted
	// setting via closebehavior.Parse, which is unit-tested independently
	// of this Wails wiring (internal/closebehavior/closebehavior_test.go).
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		raw, err := settingsService.GetCloseBehavior()
		if err != nil {
			log.Printf("stellarei: reading close behavior: %v", err)
		}
		switch closebehavior.Parse(raw) {
		case closebehavior.MinimizeToTray:
			e.Cancel()
			mainWindow.Hide()
		case closebehavior.AskEveryTime:
			e.Cancel()
			app.Event.Emit("window:confirm-close", nil)
			// The frontend shows "Close STELLAREI / Minimize to Tray /
			// Cancel" and calls WindowService.QuitApplication or
			// WindowService.MinimizeToTray directly - see
			// frontend/src/components (Phase 1 close-confirm dialog).
		case closebehavior.CloseApplication:
			app.Quit()
		}
	})

	systemTray := tray.Build(tray.Deps{
		App:         app,
		Window:      mainWindow,
		Engine:      engine,
		TimerRepo:   timerRepo,
		IconDark:    trayIconDark,
		IconLight:   trayIconLight,
		InitialMode: initialThemeMode,
		GetCloseBehavior: func() (closebehavior.Behavior, error) {
			raw, err := settingsService.GetCloseBehavior()
			return closebehavior.Parse(raw), err
		},
		SetPrivacyMode: privacyService.SetPrivacyMode,
	})

	// Wired last, once both the tray and app exist - see
	// activity.Engine.SetOnStateChange's doc comment for why this can't
	// happen at NewEngine time.
	engine.SetOnStateChange(func(state activity.State) {
		systemTray.UpdateStatus(state)
		app.Event.Emit("activity:changed", nil)
		// Payload is intentionally empty: the frontend re-fetches via
		// ActivityService.GetStatus rather than trusting an event
		// payload's shape to stay stable - see
		// frontend/src/pages/Overview.tsx.
	})

	// Wired last, once both the tray and app exist - see
	// activity.Engine.SetOnStateChange's doc comment for why this can't
	// happen at NewEngine time. Same pattern here: the tray icon switch
	// needs the tray, which needs InitialMode computed above, so the live
	// callback is wired only after Build has already set the initial icon.
	themeService.SetOnActiveThemeChange(func(mode theme.Mode, _ string) {
		systemTray.SetIconForMode(mode)
		app.Event.Emit("theme:changed", nil)
		// Payload intentionally empty - same reasoning as
		// "activity:changed": the frontend re-fetches via
		// ThemeService.GetActiveThemeName/ListThemes rather than trusting
		// an event payload's shape to stay stable.
	})

	// Global keyboard shortcuts (spec section 45): actionFuncs,
	// shortcutRegistrar, and shortcutsService were all constructed BEFORE
	// `app` above (via appRef/secondInstanceWindow indirection) so
	// shortcutsService could be in the Services list. What remains here
	// is only the initial registration, which must run after `app`
	// exists since Rebind touches the real appRef.GlobalShortcut.
	//
	// Registration failures are logged loudly (log.Printf, never
	// swallowed) so a dead shortcut is impossible to miss in a real run.
	// Initial registration: each action's persisted accelerator if the
	// person already customized it, else its built-in default - reusing
	// the exact same shortcutRegistrar.Rebind path a later frontend-
	// triggered change uses, rather than a second, separately-written
	// registration routine that could drift from it.
	for _, action := range shortcuts.Actions {
		stored, err := settingsService.GetString(services.ShortcutSettingKey(action))
		if err != nil {
			log.Printf("global shortcut: reading stored accelerator for %s: %v", action, err)
			continue
		}
		accelerator := stored
		if accelerator == "" {
			accelerator = shortcuts.DefaultAccelerator(action)
		}
		if err := shortcutRegistrar.Rebind(action, accelerator); err != nil {
			log.Printf("global shortcut: registering %q for %s failed: %v", accelerator, action, err)
		}
	}

	activityCtx, stopActivity := context.WithCancel(context.Background())
	engine.Start(activityCtx)

	// Scheduler (Phase 5): the background component that notices a
	// countdown timer reached zero or an alarm came due and fires a
	// notification - see internal/scheduler's doc comment. Wired after
	// the tray exists so OnAlarmFired/OnTimerCompleted can nudge the
	// frontend via app.Event.Emit, same "wire the callback last" pattern
	// as engine.SetOnStateChange/themeService.SetOnActiveThemeChange
	// above.
	// engine is passed in so the Scheduler can check Focus Mode's
	// blocked-apps list against the live foreground process (spec
	// section 20) - see scheduler.ActivityStatus's doc comment.
	timeScheduler := scheduler.New(timerRepo, alarmRepo, sessionRepo, categoryRepo, projectRepo, notifier, engine, scheduler.Config{
		OnAlarmFired: func(alarms.Alarm) {
			app.Event.Emit("alarm:fired", nil)
		},
		OnTimerCompleted: func(timers.Timer) {
			app.Event.Emit("timer:completed", nil)
		},
		// The audible half of "time is up", played by STELLAREI itself
		// rather than left to the OS notification's own sound - see
		// internal/alert's package doc for the Windows DND / macOS
		// authorization cases that would otherwise silence it silently.
		PlaySound: alert.Play,
	})
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	timeScheduler.Start(schedulerCtx)

	// Screenshot Memory sweeper (Phase 9, spec section 27): captures on
	// a person-configured interval and purges expired screenshots.
	// activityStatusAdapter resolves activity.State's process name to
	// an application id via the same ResolveApplicationID
	// upsert-on-lookup activityRepo already uses everywhere else, so
	// there's exactly one path that turns "a process name was seen"
	// into an applications row.
	privacySweeper := privacy.NewSweeper(
		privacyRepo, privacyRepo,
		&activityStatusAdapter{engine: engine, repo: activityRepo},
		privacy.NewSQLSettingsProvider(db),
		platform.NewScreenCapturer(),
		privacy.Config{ScreenshotDir: paths.Screenshots},
	)
	sweeperCtx, stopSweeper := context.WithCancel(context.Background())
	privacySweeper.Start(sweeperCtx)

	runErr := app.Run()

	// Explicit cleanup here, not deferred: os.Exit below would skip
	// deferred calls entirely, silently leaking the Activity Engine's
	// goroutine and leaving the database connection open on the error
	// path. Both branches below need this, so it happens once, up front.
	stopSweeper()
	privacySweeper.Stop()
	stopScheduler()
	timeScheduler.Stop()
	stopActivity()
	engine.Stop()
	db.Close()

	if runErr != nil {
		log.Println("stellarei: fatal error:", runErr)
		os.Exit(1)
	}
}
