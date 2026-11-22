package privacy

import (
	"context"
	"testing"
	"time"

	"stellarei/internal/platform"
)

// --- fakes ---------------------------------------------------------------

type fakeScreenshotStore struct {
	inserted []Screenshot
	deleted  []int64
	expired  []Screenshot
}

func (f *fakeScreenshotStore) InsertScreenshot(ctx context.Context, takenAt time.Time, filePath string, sessionID, applicationID int64, deleteAfter *time.Time) (int64, error) {
	id := int64(len(f.inserted) + 1)
	f.inserted = append(f.inserted, Screenshot{ID: id, TakenAt: takenAt, FilePath: filePath, ApplicationID: applicationID, DeleteAfter: deleteAfter})
	return id, nil
}
func (f *fakeScreenshotStore) ListExpiredScreenshots(ctx context.Context, now time.Time) ([]Screenshot, error) {
	return f.expired, nil
}
func (f *fakeScreenshotStore) DeleteScreenshot(ctx context.Context, id int64) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeExclusionStore struct {
	appPrivacy ApplicationPrivacy
	exclusions []Exclusion
}

func (f *fakeExclusionStore) ListExclusions(ctx context.Context) ([]Exclusion, error) {
	return f.exclusions, nil
}
func (f *fakeExclusionStore) GetApplicationPrivacy(ctx context.Context, applicationID int64) (ApplicationPrivacy, error) {
	return f.appPrivacy, nil
}

type fakeActivityStatusProvider struct{ status ActivityStatus }

func (f *fakeActivityStatusProvider) Status() ActivityStatus { return f.status }

type fakeSettingsProvider struct {
	enabled      bool
	intervalSecs int
	retention    RetentionDays
}

func (f *fakeSettingsProvider) ScreenshotSettings(ctx context.Context) (bool, int, RetentionDays, error) {
	return f.enabled, f.intervalSecs, f.retention, nil
}

type fakeCapturer struct {
	calls   []string
	failErr error
}

func (f *fakeCapturer) CaptureScreen(path string) error {
	f.calls = append(f.calls, path)
	return f.failErr
}

func newSweeper(store *fakeScreenshotStore, exclusions *fakeExclusionStore, activity *fakeActivityStatusProvider, settings *fakeSettingsProvider, capturer *fakeCapturer) *Sweeper {
	return NewSweeper(store, exclusions, activity, settings, capturer, Config{ScreenshotDir: "/tmp/shots"})
}

var sweepBase = time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

// --- capture ---------------------------------------------------------------

func TestTick_CapturesWhenEnabledAndDue(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe"}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 60, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)

	if len(capturer.calls) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(capturer.calls))
	}
	if len(store.inserted) != 1 {
		t.Fatalf("expected 1 inserted screenshot record, got %d", len(store.inserted))
	}
	if store.inserted[0].DeleteAfter == nil {
		t.Fatal("expected a computed DeleteAfter for RetentionSevenDays")
	}
}

func TestTick_DoesNotCaptureWhenDisabled(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe"}},
		&fakeSettingsProvider{enabled: false, intervalSecs: 60},
		capturer,
	)

	s.tick(sweepBase)

	if len(capturer.calls) != 0 {
		t.Fatal("must not capture when screenshot memory is disabled")
	}
}

func TestTick_DoesNotCaptureBeforeIntervalElapsed(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe"}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 300, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)
	s.tick(sweepBase.Add(30 * time.Second)) // well under the 300s interval

	if len(capturer.calls) != 1 {
		t.Fatalf("expected exactly 1 capture (2nd tick too soon), got %d", len(capturer.calls))
	}
}

func TestTick_CapturesAgainAfterIntervalElapses(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe"}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 300, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)
	s.tick(sweepBase.Add(301 * time.Second))

	if len(capturer.calls) != 2 {
		t.Fatalf("expected 2 captures after the interval elapsed, got %d", len(capturer.calls))
	}
}

func TestTick_PrivacyModeOrPausedBlocksCapture(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe", Paused: true}}, // Privacy Mode or manual pause - same flag
		&fakeSettingsProvider{enabled: true, intervalSecs: 60, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)

	if len(capturer.calls) != 0 {
		t.Fatal("must not capture while tracking is paused (includes Privacy Mode)")
	}
}

func TestTick_IdleBlocksCapture(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe", Idle: true}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 60, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)

	if len(capturer.calls) != 0 {
		t.Fatal("must not capture while idle")
	}
}

func TestTick_PrivacySensitiveApplicationBlocksCapture(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true, PrivacySensitive: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "1Password.exe"}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 60, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)

	if len(capturer.calls) != 0 {
		t.Fatal("must not capture a privacy_sensitive application")
	}
}

func TestTick_ExclusionListBlocksCapture(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{}
	s := newSweeper(store,
		&fakeExclusionStore{
			appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true},
			exclusions: []Exclusion{{Type: ExclusionApplication, Pattern: "Steam.exe", AppliesTo: AppliesToScreenshots}},
		},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Steam.exe"}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 60, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)

	if len(capturer.calls) != 0 {
		t.Fatal("must not capture an explicitly excluded application")
	}
}

func TestTick_CaptureFailureDoesNotRecordAScreenshot(t *testing.T) {
	store := &fakeScreenshotStore{}
	capturer := &fakeCapturer{failErr: platform.ErrUnavailable}
	s := newSweeper(store,
		&fakeExclusionStore{appPrivacy: ApplicationPrivacy{ScreenshotAllowed: true}},
		&fakeActivityStatusProvider{status: ActivityStatus{ProcessName: "Code.exe"}},
		&fakeSettingsProvider{enabled: true, intervalSecs: 60, retention: RetentionSevenDays},
		capturer,
	)

	s.tick(sweepBase)

	if len(store.inserted) != 0 {
		t.Fatal("a failed capture (e.g. no screenshot tool installed) must not record a screenshot row")
	}
}

// --- purge -------------------------------------------------------------

func TestTick_PurgesExpiredScreenshots(t *testing.T) {
	store := &fakeScreenshotStore{expired: []Screenshot{{ID: 1, FilePath: ""}, {ID: 2, FilePath: ""}}}
	s := newSweeper(store,
		&fakeExclusionStore{},
		&fakeActivityStatusProvider{status: ActivityStatus{Paused: true}}, // avoid also triggering a capture this tick
		&fakeSettingsProvider{enabled: false},
		&fakeCapturer{},
	)

	s.tick(sweepBase)

	if len(store.deleted) != 2 {
		t.Fatalf("expected 2 expired screenshots purged, got %d", len(store.deleted))
	}
}

func TestTick_NoExpiredScreenshotsPurgesNothing(t *testing.T) {
	store := &fakeScreenshotStore{}
	s := newSweeper(store,
		&fakeExclusionStore{},
		&fakeActivityStatusProvider{status: ActivityStatus{Paused: true}},
		&fakeSettingsProvider{enabled: false},
		&fakeCapturer{},
	)

	s.tick(sweepBase)

	if len(store.deleted) != 0 {
		t.Fatal("expected no deletions when nothing is expired")
	}
}
