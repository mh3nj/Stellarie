//go:build linux

package platform

import "testing"

// These tests run in this exact sandbox (no xdotool, no xprintidle, no X
// display) and exist to prove the graceful-degradation contract for real:
// missing tooling must return ErrUnavailable, never panic and never block
// past the 500ms timeout each watcher sets internally.

func TestLinuxActiveWindowWatcher_DegradesGracefully(t *testing.T) {
	w := newActiveWindowWatcher()
	_, err := w.ActiveWindow()
	if err != ErrUnavailable {
		t.Fatalf("ActiveWindow() error = %v, want ErrUnavailable (xdotool is not installed in this environment)", err)
	}
}

func TestLinuxIdleWatcher_DegradesGracefully(t *testing.T) {
	w := newIdleWatcher()
	_, err := w.IdleDuration()
	if err != ErrUnavailable {
		t.Fatalf("IdleDuration() error = %v, want ErrUnavailable (xprintidle is not installed in this environment)", err)
	}
}

func TestLinuxScreenCapturer_DegradesGracefully(t *testing.T) {
	w := newScreenCapturer()
	err := w.CaptureScreen("/tmp/stellarei-test-screenshot.png")
	if err != ErrUnavailable {
		t.Fatalf("CaptureScreen() error = %v, want ErrUnavailable (no screenshot tool is installed in this environment)", err)
	}
}
