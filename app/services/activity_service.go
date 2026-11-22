package services

import (
	"fmt"
	"time"

	"stellarei/internal/activity"
)

// ActivityStatus is the JSON shape returned to the frontend - a plain
// struct rather than exposing activity.State directly, so the Activity
// Engine's internal type can evolve without silently reshaping the
// frontend contract.
type ActivityStatus struct {
	ProcessName  string `json:"processName"`
	WindowTitle  string `json:"windowTitle"`
	Idle         bool   `json:"idle"`
	Paused       bool   `json:"paused"`
	PrivacyMode  bool   `json:"privacyMode"`  // spec section 26 - distinct from an ordinary manual pause, see activity.Engine.SetPrivacyMode's doc comment
	RunStartedAt string `json:"runStartedAt"` // RFC 3339, empty if nothing in progress
}

// ActivityService exposes the running Activity Engine to the frontend:
// live status for the Overview page, and the pause/resume controls from
// section 25. The tray's own Pause/Resume menu items call the Engine
// directly (they run in-process, no Wails binding needed) - this service
// exists purely for the frontend's JS access to the same operations.
type ActivityService struct {
	engine *activity.Engine
}

func NewActivityService(engine *activity.Engine) *ActivityService {
	return &ActivityService{engine: engine}
}

// GetStatus returns what the Activity Engine is doing right now.
func (a *ActivityService) GetStatus() ActivityStatus {
	s := a.engine.Status()
	status := ActivityStatus{
		ProcessName: s.ProcessName,
		WindowTitle: s.WindowTitle,
		Idle:        s.Idle,
		Paused:      s.Paused,
		PrivacyMode: s.PrivacyMode,
	}
	if !s.RunStartedAt.IsZero() {
		status.RunStartedAt = s.RunStartedAt.Format(time.RFC3339)
	}
	return status
}

// Pause stops tracking. minutes <= 0 means "until resumed" (section 25);
// otherwise tracking auto-resumes after that many minutes.
func (a *ActivityService) Pause(minutes int) error {
	if minutes < 0 {
		return fmt.Errorf("activity: pause minutes must be >= 0, got %d", minutes)
	}
	a.engine.Pause(time.Duration(minutes) * time.Minute)
	return nil
}

// Resume clears any pause, timed or indefinite.
func (a *ActivityService) Resume() {
	a.engine.Resume()
}
