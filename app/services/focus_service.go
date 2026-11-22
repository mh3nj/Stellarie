package services

import (
	"context"
	"fmt"
	"time"

	"stellarei/internal/activity"
	"stellarei/internal/focus"
	"stellarei/internal/sessions"
	"stellarei/internal/timers"
)

// FocusView is a focus session plus its live progress - the underlying
// timer's own fields (via TimerView's same shape) plus Focus-specific
// config and quality stats, computed fresh from real session data on
// every call (spec section 66 - never invented), never cached.
type FocusView struct {
	ID                    int64    `json:"id"`
	ProjectID             int64    `json:"projectId"`
	Label                 string   `json:"label"`
	DurationSecs          int64    `json:"durationSecs"` // 0 = open-ended
	Status                string   `json:"status"`
	StartedAt             int64    `json:"startedAt"`
	ElapsedSecs           int64    `json:"elapsedSecs"`
	RemainingSecs         int64    `json:"remainingSecs"`
	BlockedApps           []string `json:"blockedApps,omitempty"`
	BlockedWebsites       []string `json:"blockedWebsites,omitempty"`
	SuppressNotifications bool     `json:"suppressNotifications"`

	// Quality stats - all computed from internal/sessions.ListForRange
	// over [StartedAt, now-or-EndedAt), the same real activity data
	// Timeline/Analytics already use. QualityKnown is false only when
	// there's no tracked time yet (see internal/focus.Quality's doc
	// comment) - the frontend should show "not enough data yet"
	// (section 67), not a misleading 0%.
	QualityPercent    float64 `json:"qualityPercent"`
	QualityKnown      bool    `json:"qualityKnown"`
	OnTargetSecs      int64   `json:"onTargetSecs"`
	TrackedSecs       int64   `json:"trackedSecs"`
	BlockedAppSecs    int64   `json:"blockedAppSecs"`
	CurrentAppBlocked bool    `json:"currentAppBlocked"` // live - is the RIGHT NOW foreground app on the blocklist
}

// FocusService is bound to the frontend via application.NewService in
// main.go. A focus session IS a timers.TypeFocus row under the hood
// (see internal/focus's package doc) - Pause/Resume/Finish/Cancel are
// deliberately NOT duplicated here; the frontend calls TimerService for
// those using the same id, exactly as it would for any other timer.
// This service only covers what's specific to Focus: starting one with
// its blocked-apps config, and reading back the live/historical quality
// stats that config makes possible.
type FocusService struct {
	timerRepo   *timers.Repository
	sessionRepo sessions.Repository
	engine      *activity.Engine
}

func NewFocusService(timerRepo *timers.Repository, sessionRepo sessions.Repository, engine *activity.Engine) *FocusService {
	return &FocusService{timerRepo: timerRepo, sessionRepo: sessionRepo, engine: engine}
}

// Start begins a focus session. A project is required - unlike a plain
// timer, spec section 20 defines a focus session as inherently tied to
// one ("A focus session contains: project"). durationMinutes <= 0 means
// open-ended (no target, like a stopwatch - Finish ends it manually).
func (s *FocusService) Start(projectID int64, label string, durationMinutes int, blockedApps, blockedWebsites []string, suppressNotifications bool) (int64, error) {
	if projectID <= 0 {
		return 0, fmt.Errorf("focus: a project is required to start a focus session")
	}
	cfg := focus.Config{BlockedApps: blockedApps, BlockedWebsites: blockedWebsites, SuppressNotifications: suppressNotifications}
	metadataJSON, err := focus.EncodeConfig(cfg)
	if err != nil {
		return 0, err
	}
	var durationSecs int64
	if durationMinutes > 0 {
		durationSecs = int64(durationMinutes) * 60
	}
	return s.timerRepo.Create(context.Background(), timers.TypeFocus, projectID, label, durationSecs, metadataJSON, time.Now())
}

// GetActive returns the current focus session, or nil if none is
// running or paused (not an error - "no active focus session" is a
// completely normal state, e.g. the Focus page between sessions).
func (s *FocusService) GetActive() (*FocusView, error) {
	ctx := context.Background()
	active, err := s.timerRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range active {
		if t.TimerType == timers.TypeFocus {
			// ListActive is newest-first; several focus timers active at
			// once shouldn't normally happen (the frontend only offers
			// one Start Focus flow at a time), but if it ever does,
			// showing the most recently started one is the least
			// surprising choice.
			view, err := s.buildView(ctx, t, time.Now())
			return &view, err
		}
	}
	return nil, nil
}

// History returns recently completed/cancelled focus sessions, most
// recent first - each with the same real, computed-from-sessions
// quality stats as GetActive, using the session's own EndedAt as the
// window's end instead of "now" (a finished session's quality is frozen
// at when it finished, same reasoning as internal/timers.Elapsed
// freezing at EndedAt).
func (s *FocusService) History(limit int) ([]FocusView, error) {
	ctx := context.Background()
	rows, err := s.timerRepo.ListRecent(ctx, limit*4) // ListRecent isn't type-filtered; over-fetch and filter below, same pattern as GetActive's loop
	if err != nil {
		return nil, err
	}
	out := make([]FocusView, 0, limit)
	for _, t := range rows {
		if t.TimerType != timers.TypeFocus {
			continue
		}
		end := time.Now()
		if t.EndedAt != nil {
			end = *t.EndedAt
		}
		view, err := s.buildView(ctx, t, end)
		if err != nil {
			continue // one bad row (e.g. unparseable metadata from a future format) must not break the whole list
		}
		out = append(out, view)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *FocusService) buildView(ctx context.Context, t timers.Timer, now time.Time) (FocusView, error) {
	cfg, err := focus.DecodeConfig(t.MetadataJSON)
	if err != nil {
		return FocusView{}, err
	}

	elapsed := timers.Elapsed(t, now)
	remaining, _ := timers.Remaining(t, now)

	view := FocusView{
		ID: t.ID, ProjectID: t.ProjectID, Label: t.Label, DurationSecs: t.DurationSecs,
		Status: string(t.Status), StartedAt: t.StartedAt.Unix(),
		ElapsedSecs: int64(elapsed.Seconds()), RemainingSecs: int64(remaining.Seconds()),
		BlockedApps: cfg.BlockedApps, BlockedWebsites: cfg.BlockedWebsites, SuppressNotifications: cfg.SuppressNotifications,
	}

	windowEnd := now
	if t.EndedAt != nil {
		windowEnd = *t.EndedAt
	}
	windowStartUnix, windowEndUnix := t.StartedAt.Unix(), windowEnd.Unix()

	if windowEndUnix > windowStartUnix {
		if rows, err := s.sessionRepo.ListForRange(ctx, windowStartUnix, windowEndUnix); err == nil {
			var onTarget, tracked, blockedSecs int64
			for _, sv := range rows {
				if sv.Excluded {
					continue // section 63: an excluded session is excluded from every aggregate, focus quality included
				}
				start, end := int64(sv.StartedAt), int64(sv.EndedAt)
				// ListForRange returns anything OVERLAPPING the range, so
				// clip to the focus window - a session that started
				// before this focus session began must only count the
				// portion that falls within it.
				if start < windowStartUnix {
					start = windowStartUnix
				}
				if end > windowEndUnix {
					end = windowEndUnix
				}
				if end <= start {
					continue
				}
				dur := end - start
				tracked += dur
				if sv.ProjectID == t.ProjectID {
					onTarget += dur
				}
				if focus.IsBlockedApp(sv.ApplicationName, cfg.BlockedApps) {
					blockedSecs += dur
				}
			}
			view.OnTargetSecs = onTarget
			view.TrackedSecs = tracked
			view.BlockedAppSecs = blockedSecs
			if pct, ok := focus.Quality(onTarget, tracked); ok {
				view.QualityPercent = pct
				view.QualityKnown = true
			}
		}
	}

	if t.Status == timers.StatusRunning && s.engine != nil {
		state := s.engine.Status()
		view.CurrentAppBlocked = !state.Idle && !state.Paused && focus.IsBlockedApp(state.ProcessName, cfg.BlockedApps)
	}

	return view, nil
}
