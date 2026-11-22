package activity

import (
	"testing"
	"time"
)

func at(seconds int) time.Time {
	return time.Date(2026, 8, 16, 9, 0, seconds, 0, time.UTC)
}

func TestDebouncer_FirstSampleStartsRunWithoutEmitting(t *testing.T) {
	d := NewDebouncer()
	got := d.Feed(Sample{Time: at(0), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})
	if got != nil {
		t.Fatalf("Feed on first sample = %+v, want nil", got)
	}
}

func TestDebouncer_SameAppCoalescesWithoutEmitting(t *testing.T) {
	d := NewDebouncer()
	d.Feed(Sample{Time: at(0), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})

	for s := 1; s <= 10; s++ {
		got := d.Feed(Sample{Time: at(s), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})
		if got != nil {
			t.Fatalf("Feed at t=%ds coalescing same app = %+v, want nil", s, got)
		}
	}
}

func TestDebouncer_SameAppUpdatesWindowTitleWithoutEmitting(t *testing.T) {
	d := NewDebouncer()
	d.Feed(Sample{Time: at(0), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})
	d.Feed(Sample{Time: at(5), ProcessName: "Code.exe", WindowTitle: "debounce.go", Idle: false})

	// The title update should surface in whatever event eventually closes
	// this run - confirmed by flushing and checking WindowTitle below.
	ev := d.Flush(at(10))
	if ev == nil {
		t.Fatal("Flush() = nil, want a completed event")
	}
	if ev.WindowTitle != "debounce.go" {
		t.Errorf("WindowTitle = %q, want the most recently observed title %q", ev.WindowTitle, "debounce.go")
	}
	if ev.Duration != 10*time.Second {
		t.Errorf("Duration = %v, want 10s", ev.Duration)
	}
}

func TestDebouncer_AppSwitchEmitsCompletedEventAndStartsNewRun(t *testing.T) {
	d := NewDebouncer()
	d.Feed(Sample{Time: at(0), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})
	d.Feed(Sample{Time: at(5), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})

	got := d.Feed(Sample{Time: at(13), ProcessName: "chrome.exe", WindowTitle: "GitHub", Idle: false})
	if got == nil {
		t.Fatal("Feed() on app switch = nil, want a completed event for the Code.exe run")
	}
	if got.ProcessName != "Code.exe" {
		t.Errorf("completed ProcessName = %q, want Code.exe", got.ProcessName)
	}
	if got.StartedAt != at(0) {
		t.Errorf("completed StartedAt = %v, want %v", got.StartedAt, at(0))
	}
	if got.Duration != 13*time.Second {
		t.Errorf("completed Duration = %v, want 13s (closed at the switching sample's time)", got.Duration)
	}

	// The new run (chrome.exe) should now be in progress, not yet emitted.
	if again := d.Feed(Sample{Time: at(14), ProcessName: "chrome.exe", WindowTitle: "GitHub", Idle: false}); again != nil {
		t.Fatalf("Feed() continuing the new run = %+v, want nil", again)
	}
}

func TestDebouncer_IdleTransitionEmitsCompletedEvent(t *testing.T) {
	d := NewDebouncer()
	d.Feed(Sample{Time: at(0), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})

	got := d.Feed(Sample{Time: at(120), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: true})
	if got == nil {
		t.Fatal("Feed() on idle transition = nil, want a completed (non-idle) event")
	}
	if got.Idle {
		t.Error("completed event Idle = true, want false (it's the active run that just ended)")
	}
	if got.Duration != 120*time.Second {
		t.Errorf("completed Duration = %v, want 120s", got.Duration)
	}
}

func TestDebouncer_FlushWithNothingInProgressReturnsNil(t *testing.T) {
	d := NewDebouncer()
	if got := d.Flush(at(0)); got != nil {
		t.Fatalf("Flush() with nothing in progress = %+v, want nil", got)
	}
}

func TestDebouncer_FlushClosesInProgressRun(t *testing.T) {
	d := NewDebouncer()
	d.Feed(Sample{Time: at(0), ProcessName: "Code.exe", WindowTitle: "main.go", Idle: false})

	got := d.Flush(at(30))
	if got == nil {
		t.Fatal("Flush() = nil, want the in-progress run closed out")
	}
	if got.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", got.Duration)
	}

	// Flushing again with nothing in progress must be a safe no-op
	// this matters because the Engine calls Flush on both pause and
	// final shutdown, and a pause followed immediately by shutdown must
	// not double-emit.
	if again := d.Flush(at(31)); again != nil {
		t.Fatalf("second Flush() = %+v, want nil", again)
	}
}
