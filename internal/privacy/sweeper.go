// Package privacy's Sweeper is the background component that actually
// DOES screenshot memory over time - the Phase 9 analogue of
// internal/scheduler.Scheduler: a mutex-guarded ticker loop with an
// injectable Clock, narrow interfaces for every dependency so its
// decisions are unit-tested with fakes rather than a real OS or
// filesystem, following the exact same shape internal/scheduler and
// internal/activity.Engine already established.
package privacy

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"stellarei/internal/platform"
)

// ScreenshotStore is the subset of *Repository the Sweeper needs.
type ScreenshotStore interface {
	InsertScreenshot(ctx context.Context, takenAt time.Time, filePath string, sessionID, applicationID int64, deleteAfter *time.Time) (int64, error)
	ListExpiredScreenshots(ctx context.Context, now time.Time) ([]Screenshot, error)
	DeleteScreenshot(ctx context.Context, id int64) error
}

// ExclusionStore is the subset of *Repository the Sweeper needs to
// decide whether the CURRENT moment is excluded.
type ExclusionStore interface {
	ListExclusions(ctx context.Context) ([]Exclusion, error)
	GetApplicationPrivacy(ctx context.Context, applicationID int64) (ApplicationPrivacy, error)
}

// ActivityStatus is the live "what's happening right now" signal - a
// small local copy of just the fields this package needs, rather than
// importing internal/activity.State directly and risking a tighter
// coupling than this one read-only snapshot actually requires.
type ActivityStatus struct {
	ProcessName   string
	WindowTitle   string
	ApplicationID int64
	Idle          bool
	Paused        bool // includes Privacy Mode - see activity.Engine.Status's own doc comment
}

// ActivityStatusProvider is satisfied by a small adapter in main.go
// wrapping *activity.Engine.Status() plus an application-id lookup (the
// Engine's own State carries a process name, not a resolved application
// id - see main.go's wiring comment for how that gap is closed).
type ActivityStatusProvider interface {
	Status() ActivityStatus
}

// Capturer is platform.ScreenCapturer's own shape, redeclared here so
// this package depends on it structurally rather than needing every
// caller to pass a *platform.ScreenCapturer specifically - satisfied
// directly by platform.NewScreenCapturer()'s return value with no
// adapter code.
type Capturer interface {
	CaptureScreen(path string) error
}

// SettingsProvider reads the current screenshot settings fresh on every
// tick, so a mid-session change (interval, retention, on/off) takes
// effect immediately rather than needing a restart. A small interface
// (not a dependency on app/services.SettingsService directly, which
// would create an import cycle: app/services already imports
// internal/privacy) satisfied by a tiny closure-based adapter in
// main.go.
type SettingsProvider interface {
	ScreenshotSettings(ctx context.Context) (enabled bool, intervalSecs int, retention RetentionDays, err error)
}

// Config controls Sweeper behavior.
type Config struct {
	// TickInterval is how often the Sweeper checks whether it's time to
	// capture or purge - NOT the screenshot interval itself, which is a
	// person-configured setting read fresh via SettingsProvider.
	TickInterval  time.Duration
	Clock         func() time.Time
	ScreenshotDir string // where captured PNGs are written (spec section 54's "Screenshot location")
}

const defaultTickInterval = 30 * time.Second

// Sweeper is the background screenshot engine. All exported methods are
// safe for concurrent use.
type Sweeper struct {
	store      ScreenshotStore
	exclusions ExclusionStore
	activity   ActivityStatusProvider
	settings   SettingsProvider
	capturer   Capturer

	cfg Config

	mu            sync.Mutex
	wg            sync.WaitGroup
	cancel        context.CancelFunc
	lastCaptureAt time.Time
}

func NewSweeper(store ScreenshotStore, exclusions ExclusionStore, activity ActivityStatusProvider, settings SettingsProvider, capturer Capturer, cfg Config) *Sweeper {
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = defaultTickInterval
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Sweeper{store: store, exclusions: exclusions, activity: activity, settings: settings, capturer: capturer, cfg: cfg}
}

func (s *Sweeper) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go s.run(runCtx)
}

func (s *Sweeper) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Sweeper) run(ctx context.Context) {
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
// internal/scheduler.Scheduler.tick.
func (s *Sweeper) tick(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.maybeCaptureLocked(ctx, now)
	s.purgeExpired(ctx, now)
}

func (s *Sweeper) maybeCaptureLocked(ctx context.Context, now time.Time) {
	enabled, intervalSecs, retention, err := s.settings.ScreenshotSettings(ctx)
	if err != nil {
		log.Printf("privacy: reading screenshot settings: %v", err)
		return
	}
	if !enabled || intervalSecs <= 0 {
		return
	}
	if !s.lastCaptureAt.IsZero() && now.Sub(s.lastCaptureAt) < time.Duration(intervalSecs)*time.Second {
		return // not due yet
	}

	status := s.activity.Status()
	if status.Paused || status.Idle {
		// Section 26: Privacy Mode (folded into Paused, same as
		// internal/activity.Engine.Status) always blocks capture.
		// Section 27 doesn't ask for idle-time screenshots either - an
		// empty desk isn't something worth remembering.
		return
	}

	appPrivacy, err := s.exclusions.GetApplicationPrivacy(ctx, status.ApplicationID)
	if err != nil {
		log.Printf("privacy: getting application privacy: %v", err)
		return
	}
	exclusionRows, err := s.exclusions.ListExclusions(ctx)
	if err != nil {
		log.Printf("privacy: listing exclusions: %v", err)
		return
	}
	blockCtx := Context{
		ProcessName: status.ProcessName, WindowTitle: status.WindowTitle,
		PrivacySensitive: appPrivacy.PrivacySensitive, ScreenshotAllowed: appPrivacy.ScreenshotAllowed,
	}
	if IsScreenshotBlocked(blockCtx, exclusionRows) {
		return
	}

	path := screenshotPath(s.cfg.ScreenshotDir, now)
	if err := s.capturer.CaptureScreen(path); err != nil {
		if err != platform.ErrUnavailable {
			log.Printf("privacy: capturing screenshot: %v", err)
		}
		// Section 56: a capture failure (no screenshot tool installed,
		// permission denied, etc.) must never stop the rest of
		// STELLAREI - just skip this interval and try again next tick.
		return
	}

	deleteAfter := ComputeDeleteAfter(now, retention)
	if _, err := s.store.InsertScreenshot(ctx, now, path, 0, status.ApplicationID, deleteAfter); err != nil {
		log.Printf("privacy: recording screenshot: %v", err)
		return
	}
	s.lastCaptureAt = now
}

func (s *Sweeper) purgeExpired(ctx context.Context, now time.Time) {
	expired, err := s.store.ListExpiredScreenshots(ctx, now)
	if err != nil {
		log.Printf("privacy: listing expired screenshots: %v", err)
		return
	}
	for _, sc := range expired {
		if sc.FilePath != "" {
			if err := os.Remove(sc.FilePath); err != nil && !os.IsNotExist(err) {
				log.Printf("privacy: removing screenshot file %s: %v", sc.FilePath, err)
				// Still remove the DB row below - a dangling file that
				// failed to delete is a lesser problem than a phantom
				// history entry the person can never clear.
			}
		}
		if err := s.store.DeleteScreenshot(ctx, sc.ID); err != nil {
			log.Printf("privacy: deleting expired screenshot %d: %v", sc.ID, err)
		}
	}
}

func screenshotPath(dir string, at time.Time) string {
	return dir + "/" + at.UTC().Format("20060102-150405") + ".png"
}
