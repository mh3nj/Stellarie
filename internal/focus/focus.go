// Package focus implements Focus Mode (spec section 20): a bounded work
// session tied to a project, with an optional list of blocked
// applications and a live "focus quality" score computed from real
// activity data - never invented (section 66's "never invent
// statistics" applies here as much as it does to Analytics).
//
// A focus session IS a timers.TypeFocus timer under the hood, reusing
// internal/timers' pause/resume/complete machinery rather than
// duplicating it (the same "one mechanism, several timer types" shape
// pomodoro.go already established for Pomodoro). This package adds only
// what's specific to Focus: the blocked-apps config (round-tripped
// through the timer's own metadata_json, exactly like PomodoroState),
// blocked-app matching, and the quality calculation. Persisting/
// starting/pausing/resuming/finishing the underlying timer stays
// internal/timers' job.
package focus

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Config is a focus session's own settings beyond what a generic timer
// already has (project, duration) - round-tripped through
// Timer.MetadataJSON.
//
// BlockedWebsites is accepted and stored, but genuinely not enforced:
// STELLAREI has no browser URL/domain tracking yet (that's Phase 9,
// spec section 28, currently OFF-by-default and unbuilt) so there is no
// real signal to check a website block against. Storing it now means a
// focus session's config doesn't need a schema-breaking change once
// Phase 9 lands; a website in this list simply has no effect until
// then. Never silently faking website blocking is the same discipline
// section 65's own "domain rule never matches until Phase 9" comment
// already established for internal/projects.
type Config struct {
	BlockedApps           []string `json:"blockedApps,omitempty"`
	BlockedWebsites       []string `json:"blockedWebsites,omitempty"`
	SuppressNotifications bool     `json:"suppressNotifications,omitempty"`
}

func EncodeConfig(c Config) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("focus: encoding config: %w", err)
	}
	return string(b), nil
}

// DecodeConfig returns a zero Config (no blocking, notifications not
// suppressed) for empty metadata rather than an error - a focus timer
// created with no blocking configured is a completely normal, valid
// focus session, not a malformed one.
func DecodeConfig(metadataJSON string) (Config, error) {
	if metadataJSON == "" {
		return Config{}, nil
	}
	var c Config
	if err := json.Unmarshal([]byte(metadataJSON), &c); err != nil {
		return Config{}, fmt.Errorf("focus: decoding config: %w", err)
	}
	return c, nil
}

// IsBlockedApp reports whether processName matches any entry in
// blockedApps. Case-insensitive exact match - the same convention
// internal/projects' rule evaluator uses for its own process-name rules
// (rules.go's ProcessNameRule), so a person typing "Steam.exe" once
// behaves identically whether they're writing a project rule or a
// focus session's blocklist.
func IsBlockedApp(processName string, blockedApps []string) bool {
	for _, b := range blockedApps {
		if strings.EqualFold(processName, b) {
			return true
		}
	}
	return false
}

// Quality is section 20's "Focus quality: 92%" - the fraction of
// TRACKED time (not wall-clock time) actually spent on the focus
// session's own project. Idle/untracked gaps are excluded from both
// sides, the same way Budgets/Analytics already exclude idle from every
// sum they compute - stepping away from the computer isn't a
// distraction, it's just untracked, and shouldn't be held against the
// score any more than it's held against a budget.
//
// Returns ok=false when there's no tracked time yet - a percentage
// computed from zero data isn't a real "92%", it's "not enough data
// yet" (section 66/67), and the caller (FocusService) is responsible
// for presenting that distinction rather than showing a misleading 0%.
func Quality(onTargetSecs, totalTrackedSecs int64) (percent float64, ok bool) {
	if totalTrackedSecs <= 0 {
		return 0, false
	}
	p := float64(onTargetSecs) / float64(totalTrackedSecs) * 100
	if p > 100 {
		p = 100 // defensive clamp: on-target is a subset of tracked by construction, but a corrupted/edited row shouldn't be able to show >100%
	}
	if p < 0 {
		p = 0
	}
	return p, true
}
