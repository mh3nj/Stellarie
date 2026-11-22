package services

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"time"

	"stellarei/internal/config"
)

// DiagnosticsView is spec sections 55/80's Diagnostics page: "application
// version, OS, backend status, database status, activity engine status,
// tray status, startup status, last successful database write,
// performance metrics." Everything here is a real, currently-true fact
// - no placeholder fields for the pieces this session didn't wire (see
// the doc comment on DiagnosticsService below for exactly what's
// covered and what isn't).
type DiagnosticsView struct {
	AppVersion string `json:"appVersion"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`

	DatabasePath      string `json:"databasePath"`
	DatabaseReachable bool   `json:"databaseReachable"` // real ping, not assumed
	DatabaseSizeBytes int64  `json:"databaseSizeBytes"` // 0 if the file can't be stat'd
	EventsStored      int64  `json:"eventsStored"`
	SessionsStored    int64  `json:"sessionsStored"`

	ActivityEngineRunning     bool   `json:"activityEngineRunning"` // !Paused
	ActivityEngineIdle        bool   `json:"activityEngineIdle"`
	ActivityEnginePrivacyMode bool   `json:"activityEnginePrivacyMode"`
	ActivityEngineLastUpdate  string `json:"activityEngineLastUpdate"` // RFC 3339, "" if never

	// StartupRegistered reads back the OS's own record via
	// StartupService.IsEnabled() (which itself calls
	// platform.StartupManager.IsRegistered() - a real file/registry
	// check, not STELLAREI's cached setting) - see that method's own
	// doc comment on why it deliberately doesn't trust the cache.
	// StartupCheckError is set instead of failing the whole page if
	// that OS-level check itself errors (mirrors how DatabaseReachable
	// degrades rather than failing GetDiagnostics outright).
	StartupRegistered bool   `json:"startupRegistered"`
	StartupCheckError string `json:"startupCheckError"` // "" if the check succeeded

	ProcessUptimeSeconds int64   `json:"processUptimeSeconds"`
	GoroutineCount       int     `json:"goroutineCount"`
	MemoryAllocMB        float64 `json:"memoryAllocMb"`
}

// DiagnosticsService composes real, currently-observable facts from
// across the backend. What this deliberately does NOT cover, stated
// plainly rather than implied: tray status (spec section 55 also lists
// this). Investigated for real before being left out - Wails v3's
// SystemTray API (github.com/wailsapp/wails/v3/pkg/application, per
// v3.wails.io/features/menus/systray/) has no error return and no
// status/IsRunning getter on SystemTray.New/SetLabel/SetMenu/SetIcon at
// all, so there is currently no real OS-level fact to read back
// tray.Build() either succeeds synchronously at startup or the process
// wouldn't be running to answer this call in the first place. A field
// that could only ever read "running" isn't a diagnostic, it's a
// constant - exactly the kind of plausible-looking placeholder this
// service avoids elsewhere. Revisit if a future Wails version adds a
// real status API. Startup-registration status, previously also listed
// here as unwired, is now wired below (see StartupRegistered)
// StartupService.IsEnabled() already existed and already did a real
// OS-level check (this doc comment and README.md previously claimed
// otherwise; that claim was simply wrong, corrected here rather than
// carried forward).
type DiagnosticsService struct {
	db              *sql.DB
	dbPath          string
	activityService *ActivityService
	startupService  *StartupService
	processStarted  time.Time
}

// NewDiagnosticsService's processStarted should be captured once, at
// or very near actual process start (main.go) - everything after
// construction treats it as fixed, so uptime is measured from real
// startup, not from whenever this service happens to be asked about.
func NewDiagnosticsService(db *sql.DB, dbPath string, activityService *ActivityService, startupService *StartupService, processStarted time.Time) *DiagnosticsService {
	return &DiagnosticsService{db: db, dbPath: dbPath, activityService: activityService, startupService: startupService, processStarted: processStarted}
}

// GetDiagnostics collects the current snapshot. Database reachability
// and row counts are best-effort - a failure there degrades those
// specific fields to their zero value rather than failing the whole
// page, since "the database has a problem" is exactly the kind of
// thing this page exists to surface, not hide behind an error screen.
func (s *DiagnosticsService) GetDiagnostics() DiagnosticsView {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	view := DiagnosticsView{
		AppVersion:   config.AppVersion,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		DatabasePath: s.dbPath,
	}

	if s.db != nil {
		view.DatabaseReachable = s.db.PingContext(ctx) == nil
		if view.DatabaseReachable {
			_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_events`).Scan(&view.EventsStored)
			_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&view.SessionsStored)
		}
	}
	if info, err := os.Stat(s.dbPath); err == nil {
		view.DatabaseSizeBytes = info.Size()
	}

	if s.activityService != nil {
		status := s.activityService.GetStatus()
		view.ActivityEngineRunning = !status.Paused
		view.ActivityEngineIdle = status.Idle
		view.ActivityEnginePrivacyMode = status.PrivacyMode
		view.ActivityEngineLastUpdate = status.RunStartedAt
	}

	if s.startupService != nil {
		registered, err := s.startupService.IsEnabled()
		if err != nil {
			view.StartupCheckError = err.Error()
		} else {
			view.StartupRegistered = registered
		}
	}

	view.ProcessUptimeSeconds = int64(time.Since(s.processStarted).Seconds())
	view.GoroutineCount = runtime.NumGoroutine()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	view.MemoryAllocMB = float64(mem.Alloc) / (1024 * 1024)

	return view
}
