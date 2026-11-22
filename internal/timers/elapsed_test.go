package timers

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

func TestElapsed_RunningNoPause(t *testing.T) {
	tm := Timer{StartedAt: base, Status: StatusRunning}
	got := Elapsed(tm, base.Add(10*time.Minute))
	if got != 10*time.Minute {
		t.Fatalf("got %v, want 10m", got)
	}
}

func TestElapsed_SubtractsAccumulatedPause(t *testing.T) {
	tm := Timer{
		StartedAt:            base,
		Status:               StatusRunning,
		AccumulatedPauseSecs: int64((3 * time.Minute).Seconds()),
	}
	// 10 minutes of wall-clock time, 3 of which were a past (already
	// resumed) pause, should leave 7 minutes of actual run time.
	got := Elapsed(tm, base.Add(10*time.Minute))
	if got != 7*time.Minute {
		t.Fatalf("got %v, want 7m", got)
	}
}

func TestElapsed_CurrentlyPausedStopsAccruing(t *testing.T) {
	pausedAt := base.Add(6 * time.Minute)
	tm := Timer{StartedAt: base, Status: StatusPaused, PausedAt: &pausedAt}

	// Paused at +6m; asking "what's elapsed" at +20m (14 minutes into the
	// pause) must still read 6m, not 20m - a paused timer doesn't keep
	// ticking just because wall-clock time passes.
	got := Elapsed(tm, base.Add(20*time.Minute))
	if got != 6*time.Minute {
		t.Fatalf("got %v, want 6m (frozen at pause point)", got)
	}
}

func TestElapsed_PauseThenResumeThenPauseAgain(t *testing.T) {
	// Simulates two separate pauses: the first is already folded into
	// AccumulatedPauseSecs (2m), the second is the CURRENT pause
	// (PausedAt set). Timeline: run 0-5 (5m), pause 5-7 (2m, already
	// accumulated), run 7-12 (5m), pause again at 12 (current).
	pausedAt := base.Add(12 * time.Minute)
	tm := Timer{
		StartedAt:            base,
		Status:               StatusPaused,
		AccumulatedPauseSecs: int64((2 * time.Minute).Seconds()),
		PausedAt:             &pausedAt,
	}
	got := Elapsed(tm, base.Add(30*time.Minute)) // long after the 2nd pause started
	want := 10 * time.Minute                     // 5m + 5m of actual running time
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestElapsed_CompletedFreezesAtEndedAt(t *testing.T) {
	ended := base.Add(25 * time.Minute)
	tm := Timer{StartedAt: base, Status: StatusCompleted, EndedAt: &ended}

	// Asking "what's elapsed" long after completion must still return the
	// duration that had actually elapsed when it completed.
	got := Elapsed(tm, base.Add(3*time.Hour))
	if got != 25*time.Minute {
		t.Fatalf("got %v, want 25m (frozen at EndedAt)", got)
	}
}

func TestElapsed_NeverNegative(t *testing.T) {
	// Pathological/corrupted row: accumulated pause exceeds actual
	// wall-clock elapsed. Must clamp to zero, never go negative.
	tm := Timer{
		StartedAt:            base,
		Status:               StatusRunning,
		AccumulatedPauseSecs: int64((999 * time.Hour).Seconds()),
	}
	got := Elapsed(tm, base.Add(time.Minute))
	if got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestRemaining_Stopwatch_NeverComplete(t *testing.T) {
	tm := Timer{StartedAt: base, Status: StatusRunning, DurationSecs: 0}
	remaining, complete := Remaining(tm, base.Add(100*time.Hour))
	if complete {
		t.Fatal("a stopwatch (no DurationSecs) must never report complete")
	}
	if remaining != 0 {
		t.Fatalf("remaining = %v, want 0 for a stopwatch", remaining)
	}
}

func TestRemaining_CountdownInProgress(t *testing.T) {
	tm := Timer{StartedAt: base, Status: StatusRunning, DurationSecs: int64((25 * time.Minute).Seconds())}
	remaining, complete := Remaining(tm, base.Add(10*time.Minute))
	if complete {
		t.Fatal("15 minutes remain, must not be complete")
	}
	if remaining != 15*time.Minute {
		t.Fatalf("remaining = %v, want 15m", remaining)
	}
}

func TestRemaining_CountdownExactlyAtTarget(t *testing.T) {
	tm := Timer{StartedAt: base, Status: StatusRunning, DurationSecs: int64((25 * time.Minute).Seconds())}
	remaining, complete := Remaining(tm, base.Add(25*time.Minute))
	if !complete {
		t.Fatal("elapsed == target must count as complete, not '0 remaining but not done'")
	}
	if remaining != 0 {
		t.Fatalf("remaining = %v, want 0", remaining)
	}
}

func TestRemaining_CountdownPastTarget(t *testing.T) {
	tm := Timer{StartedAt: base, Status: StatusRunning, DurationSecs: int64((25 * time.Minute).Seconds())}
	remaining, complete := Remaining(tm, base.Add(90*time.Minute))
	if !complete {
		t.Fatal("well past target must be complete")
	}
	if remaining != 0 {
		t.Fatalf("remaining = %v, want 0 (never negative)", remaining)
	}
}
