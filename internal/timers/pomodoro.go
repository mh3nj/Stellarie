package timers

import (
	"encoding/json"
	"fmt"
	"time"
)

// PomodoroPhase is which stage of the Pomodoro cycle a timer is in.
type PomodoroPhase string

const (
	PhaseWork       PomodoroPhase = "work"
	PhaseShortBreak PomodoroPhase = "short_break"
	PhaseLongBreak  PomodoroPhase = "long_break"
)

// PomodoroConfig controls cycle timing. Zero-value fields fall back to
// the classic Pomodoro defaults in NewPomodoroConfig - callers normally
// only override what the user actually changed in Settings.
type PomodoroConfig struct {
	WorkMinutes          int
	ShortBreakMinutes    int
	LongBreakMinutes     int
	CyclesUntilLongBreak int // work sessions completed before a long break
}

// NewPomodoroConfig returns the classic 25/5/15, 4-cycle Pomodoro
// defaults, with any explicitly-set (non-zero) field in override kept.
func NewPomodoroConfig(override PomodoroConfig) PomodoroConfig {
	cfg := PomodoroConfig{WorkMinutes: 25, ShortBreakMinutes: 5, LongBreakMinutes: 15, CyclesUntilLongBreak: 4}
	if override.WorkMinutes > 0 {
		cfg.WorkMinutes = override.WorkMinutes
	}
	if override.ShortBreakMinutes > 0 {
		cfg.ShortBreakMinutes = override.ShortBreakMinutes
	}
	if override.LongBreakMinutes > 0 {
		cfg.LongBreakMinutes = override.LongBreakMinutes
	}
	if override.CyclesUntilLongBreak > 0 {
		cfg.CyclesUntilLongBreak = override.CyclesUntilLongBreak
	}
	return cfg
}

// PomodoroState is the piece of Pomodoro progress that has to survive
// between one phase's timer row completing and the next phase's timer
// row being created - round-tripped through Timer.MetadataJSON (see
// EncodePomodoroState/DecodePomodoroState) rather than given its own
// table, since it's meaningless outside the context of a specific
// pomodoro timer chain.
type PomodoroState struct {
	Phase               PomodoroPhase  `json:"phase"`
	CompletedWorkCycles int            `json:"completedWorkCycles"`
	Config              PomodoroConfig `json:"config"`
}

// DurationFor returns how long the given phase should run for, per s's
// own config.
func (s PomodoroState) DurationFor(phase PomodoroPhase) time.Duration {
	switch phase {
	case PhaseShortBreak:
		return time.Duration(s.Config.ShortBreakMinutes) * time.Minute
	case PhaseLongBreak:
		return time.Duration(s.Config.LongBreakMinutes) * time.Minute
	default: // PhaseWork
		return time.Duration(s.Config.WorkMinutes) * time.Minute
	}
}

// NextPhase computes what comes after the current phase completes. Pure
// state transition, no timers/clock involved, so it's exhaustively
// testable - internal/scheduler calls this exactly once per completed
// Pomodoro timer, then starts a new timer row for the returned phase.
//
// Rules (standard Pomodoro technique):
//   - Work completing advances CompletedWorkCycles. If that count is now
//     a multiple of CyclesUntilLongBreak, the next phase is a long break;
//     otherwise a short break.
//   - Either break phase completing always returns to work, with the
//     cycle count unchanged (breaks don't count as cycles).
func NextPhase(s PomodoroState) PomodoroState {
	cfg := NewPomodoroConfig(s.Config) // defends against a zero-value Config sneaking in from an old/corrupt row
	s.Config = cfg

	switch s.Phase {
	case PhaseShortBreak, PhaseLongBreak:
		s.Phase = PhaseWork
		return s
	default: // PhaseWork (and any unrecognized value defaults to treating it as work having just finished)
		s.CompletedWorkCycles++
		if s.CompletedWorkCycles%cfg.CyclesUntilLongBreak == 0 {
			s.Phase = PhaseLongBreak
		} else {
			s.Phase = PhaseShortBreak
		}
		return s
	}
}

// EncodePomodoroState/DecodePomodoroState round-trip a PomodoroState
// through Timer.MetadataJSON. Kept as named functions (not
// json.Marshal/Unmarshal called inline at every call site) so the
// metadata_json column's shape has exactly one place that defines it
// see internal/timers/repository.go's Create/UpdateMetadata.
func EncodePomodoroState(s PomodoroState) (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("timers: encoding pomodoro state: %w", err)
	}
	return string(b), nil
}

func DecodePomodoroState(metadataJSON string) (PomodoroState, error) {
	var s PomodoroState
	if metadataJSON == "" {
		return PomodoroState{}, fmt.Errorf("timers: empty metadata, not a pomodoro timer")
	}
	if err := json.Unmarshal([]byte(metadataJSON), &s); err != nil {
		return PomodoroState{}, fmt.Errorf("timers: decoding pomodoro state: %w", err)
	}
	return s, nil
}
