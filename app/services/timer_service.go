package services

import (
	"context"
	"fmt"
	"time"

	"stellarei/internal/timers"
)

// TimerView is a timer plus its live elapsed/remaining time, computed
// fresh on every call (never stored - same "compute on read" philosophy
// BudgetStatus/GoalStatus use) so the frontend never has to reconcile a
// stale server-side ticker with its own clock.
type TimerView struct {
	ID                 int64  `json:"id"`
	TimerType          string `json:"timerType"`
	ProjectID          int64  `json:"projectId"`
	Label              string `json:"label"`
	DurationSecs       int64  `json:"durationSecs"` // 0 = no target (stopwatch)
	Status             string `json:"status"`
	StartedAt          int64  `json:"startedAt"` // unix seconds
	ElapsedSecs        int64  `json:"elapsedSecs"`
	RemainingSecs      int64  `json:"remainingSecs"`
	PomodoroPhase      string `json:"pomodoroPhase,omitempty"` // "" unless TimerType == "pomodoro"
	PomodoroCyclesDone int    `json:"pomodoroCyclesDone,omitempty"`
}

// TimerService is bound to the frontend via application.NewService in
// main.go. Thin adapter over internal/timers.Repository, matching
// BudgetService/GoalService's shape - the actual elapsed/pause-aware
// math lives in internal/timers (pure, unit-tested), not here.
type TimerService struct {
	repo *timers.Repository
}

func NewTimerService(repo *timers.Repository) *TimerService {
	return &TimerService{repo: repo}
}

func toTimerView(t timers.Timer, now time.Time) TimerView {
	elapsed := timers.Elapsed(t, now)
	remaining, _ := timers.Remaining(t, now)

	v := TimerView{
		ID:            t.ID,
		TimerType:     string(t.TimerType),
		ProjectID:     t.ProjectID,
		Label:         t.Label,
		DurationSecs:  t.DurationSecs,
		Status:        string(t.Status),
		StartedAt:     t.StartedAt.Unix(),
		ElapsedSecs:   int64(elapsed.Seconds()),
		RemainingSecs: int64(remaining.Seconds()),
	}
	if t.TimerType == timers.TypePomodoro {
		if state, err := timers.DecodePomodoroState(t.MetadataJSON); err == nil {
			v.PomodoroPhase = string(state.Phase)
			v.PomodoroCyclesDone = state.CompletedWorkCycles
		}
	}
	return v
}

// ListActive returns every running or paused timer.
func (s *TimerService) ListActive() ([]TimerView, error) {
	rows, err := s.repo.ListActive(context.Background())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]TimerView, 0, len(rows))
	for _, t := range rows {
		out = append(out, toTimerView(t, now))
	}
	return out, nil
}

// ListRecent returns recently completed/cancelled timers (history).
func (s *TimerService) ListRecent(limit int) ([]TimerView, error) {
	rows, err := s.repo.ListRecent(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]TimerView, 0, len(rows))
	for _, t := range rows {
		out = append(out, toTimerView(t, now))
	}
	return out, nil
}

// StartStopwatch starts an open-ended timer with no target duration.
func (s *TimerService) StartStopwatch(projectID int64, label string) (int64, error) {
	return s.repo.Create(context.Background(), timers.TypeStopwatch, projectID, label, 0, "", time.Now())
}

// StartCountdown starts a timer that completes automatically once
// durationMinutes have elapsed (internal/scheduler notices this - see
// that package's doc comment). timerType must be one of "countdown",
// "focus", "project", or "custom" - anything with a fixed target that
// ISN'T Pomodoro (use StartPomodoro for that, since it needs the initial
// phase-state metadata Pomodoro completions chain off of).
func (s *TimerService) StartCountdown(timerType string, projectID int64, label string, durationMinutes int) (int64, error) {
	switch timers.Type(timerType) {
	case timers.TypeCountdown, timers.TypeFocus, timers.TypeProject, timers.TypeCustom, timers.TypeInterval:
	default:
		return 0, fmt.Errorf("timers: StartCountdown doesn't accept type %q (use StartStopwatch or StartPomodoro)", timerType)
	}
	if durationMinutes <= 0 {
		return 0, fmt.Errorf("timers: duration must be positive")
	}
	return s.repo.Create(context.Background(), timers.Type(timerType), projectID, label, int64(durationMinutes*60), "", time.Now())
}

// StartPomodoro starts a Pomodoro timer chain's first (work) phase.
// workMinutes/shortBreakMinutes/longBreakMinutes/cyclesUntilLongBreak of
// 0 fall back to the classic 25/5/15/4 defaults (see
// timers.NewPomodoroConfig) - the frontend only needs to pass non-zero
// values for whatever the person actually customized in Settings.
func (s *TimerService) StartPomodoro(projectID int64, label string, workMinutes, shortBreakMinutes, longBreakMinutes, cyclesUntilLongBreak int) (int64, error) {
	cfg := timers.NewPomodoroConfig(timers.PomodoroConfig{
		WorkMinutes: workMinutes, ShortBreakMinutes: shortBreakMinutes,
		LongBreakMinutes: longBreakMinutes, CyclesUntilLongBreak: cyclesUntilLongBreak,
	})
	state := timers.PomodoroState{Phase: timers.PhaseWork, Config: cfg}
	metadataJSON, err := timers.EncodePomodoroState(state)
	if err != nil {
		return 0, err
	}
	return s.repo.Create(context.Background(), timers.TypePomodoro, projectID, label, int64(cfg.WorkMinutes*60), metadataJSON, time.Now())
}

func (s *TimerService) Pause(id int64) error {
	return s.repo.Pause(context.Background(), id, time.Now())
}

func (s *TimerService) Resume(id int64) error {
	return s.repo.Resume(context.Background(), id, time.Now())
}

// Finish marks a timer completed early (e.g. a stopwatch someone stops
// manually, or a countdown finished ahead of its target) - distinct from
// internal/scheduler's automatic completion, but the same underlying
// repository call.
func (s *TimerService) Finish(id int64) error {
	return s.repo.Complete(context.Background(), id, time.Now())
}

func (s *TimerService) Cancel(id int64) error {
	return s.repo.Cancel(context.Background(), id, time.Now())
}
