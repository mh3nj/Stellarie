package timers

import "testing"

func defaultState() PomodoroState {
	return PomodoroState{Phase: PhaseWork, Config: NewPomodoroConfig(PomodoroConfig{})}
}

func TestNewPomodoroConfig_Defaults(t *testing.T) {
	cfg := NewPomodoroConfig(PomodoroConfig{})
	if cfg.WorkMinutes != 25 || cfg.ShortBreakMinutes != 5 || cfg.LongBreakMinutes != 15 || cfg.CyclesUntilLongBreak != 4 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestNewPomodoroConfig_PartialOverride(t *testing.T) {
	// Only WorkMinutes customized - everything else should still fall
	// back to the classic default, not zero out.
	cfg := NewPomodoroConfig(PomodoroConfig{WorkMinutes: 50})
	if cfg.WorkMinutes != 50 {
		t.Fatalf("WorkMinutes = %d, want 50", cfg.WorkMinutes)
	}
	if cfg.ShortBreakMinutes != 5 || cfg.LongBreakMinutes != 15 || cfg.CyclesUntilLongBreak != 4 {
		t.Fatalf("non-overridden fields should keep defaults: %+v", cfg)
	}
}

func TestNextPhase_WorkToShortBreak(t *testing.T) {
	s := defaultState()
	next := NextPhase(s)
	if next.Phase != PhaseShortBreak {
		t.Fatalf("phase = %q, want short_break (1st cycle of 4)", next.Phase)
	}
	if next.CompletedWorkCycles != 1 {
		t.Fatalf("completed cycles = %d, want 1", next.CompletedWorkCycles)
	}
}

func TestNextPhase_ShortBreakBackToWork(t *testing.T) {
	s := PomodoroState{Phase: PhaseShortBreak, CompletedWorkCycles: 1, Config: NewPomodoroConfig(PomodoroConfig{})}
	next := NextPhase(s)
	if next.Phase != PhaseWork {
		t.Fatalf("phase = %q, want work", next.Phase)
	}
	if next.CompletedWorkCycles != 1 {
		t.Fatal("a break completing must not itself increment the work-cycle count")
	}
}

func TestNextPhase_FourthWorkCycleTriggersLongBreak(t *testing.T) {
	s := PomodoroState{Phase: PhaseWork, CompletedWorkCycles: 3, Config: NewPomodoroConfig(PomodoroConfig{})}
	next := NextPhase(s)
	if next.Phase != PhaseLongBreak {
		t.Fatalf("phase = %q, want long_break on the 4th completed work cycle", next.Phase)
	}
	if next.CompletedWorkCycles != 4 {
		t.Fatalf("completed cycles = %d, want 4", next.CompletedWorkCycles)
	}
}

func TestNextPhase_LongBreakBackToWorkResetsNothing(t *testing.T) {
	s := PomodoroState{Phase: PhaseLongBreak, CompletedWorkCycles: 4, Config: NewPomodoroConfig(PomodoroConfig{})}
	next := NextPhase(s)
	if next.Phase != PhaseWork {
		t.Fatalf("phase = %q, want work", next.Phase)
	}
	if next.CompletedWorkCycles != 4 {
		t.Fatalf("completed cycles = %d, want 4 (unchanged - a new cycle count starts accruing after this)", next.CompletedWorkCycles)
	}
}

func TestNextPhase_FullTwoRoundTrip(t *testing.T) {
	// Simulates a full 4-cycle Pomodoro round from scratch:
	// work -> short -> work -> short -> work -> short -> work -> long -> work...
	s := defaultState()
	wantSequence := []PomodoroPhase{
		PhaseShortBreak, PhaseWork, PhaseShortBreak, PhaseWork,
		PhaseShortBreak, PhaseWork, PhaseLongBreak, PhaseWork,
	}
	for i, want := range wantSequence {
		s = NextPhase(s)
		if s.Phase != want {
			t.Fatalf("step %d: phase = %q, want %q", i, s.Phase, want)
		}
	}
	if s.CompletedWorkCycles != 4 {
		t.Fatalf("after one full round, completed cycles = %d, want 4", s.CompletedWorkCycles)
	}
}

func TestNextPhase_CustomCyclesUntilLongBreak(t *testing.T) {
	s := PomodoroState{Phase: PhaseWork, CompletedWorkCycles: 1, Config: NewPomodoroConfig(PomodoroConfig{CyclesUntilLongBreak: 2})}
	next := NextPhase(s)
	if next.Phase != PhaseLongBreak {
		t.Fatalf("phase = %q, want long_break (2nd cycle of a 2-cycle config)", next.Phase)
	}
}

func TestDurationFor(t *testing.T) {
	s := PomodoroState{Config: NewPomodoroConfig(PomodoroConfig{WorkMinutes: 50, ShortBreakMinutes: 10, LongBreakMinutes: 30})}
	if s.DurationFor(PhaseWork).Minutes() != 50 {
		t.Fatalf("work duration wrong: %v", s.DurationFor(PhaseWork))
	}
	if s.DurationFor(PhaseShortBreak).Minutes() != 10 {
		t.Fatalf("short break duration wrong: %v", s.DurationFor(PhaseShortBreak))
	}
	if s.DurationFor(PhaseLongBreak).Minutes() != 30 {
		t.Fatalf("long break duration wrong: %v", s.DurationFor(PhaseLongBreak))
	}
}

func TestEncodeDecodePomodoroState_RoundTrip(t *testing.T) {
	s := PomodoroState{Phase: PhaseShortBreak, CompletedWorkCycles: 2, Config: NewPomodoroConfig(PomodoroConfig{WorkMinutes: 45})}
	encoded, err := EncodePomodoroState(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodePomodoroState(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded != s {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded, s)
	}
}

func TestDecodePomodoroState_EmptyIsError(t *testing.T) {
	if _, err := DecodePomodoroState(""); err == nil {
		t.Fatal("expected an error decoding empty metadata")
	}
}
