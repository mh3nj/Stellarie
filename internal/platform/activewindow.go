package platform

import "errors"

// ErrUnavailable is returned by an OS-specific implementation when it
// genuinely cannot answer right now - e.g. no window manager running, no
// window currently focused, or (on Linux) a Wayland session where the
// X11-based fallback doesn't apply. The Activity Engine treats this as
// "no sample this tick", not a fatal error (section 56: one subsystem's
// failure must never take down tracking as a whole).
var ErrUnavailable = errors.New("platform: unavailable")

// ActiveWindowInfo describes whatever is in the foreground at the moment
// it was sampled. Fields are intentionally the minimum section 6 asks
// for - no window contents, no clipboard, nothing beyond identifying
// *which* application and window title are active.
type ActiveWindowInfo struct {
	ProcessName string // e.g. "Code.exe" (Windows) or "Code" (macOS/Linux)
	WindowTitle string
	PID         int
}

// ActiveWindowWatcher reports the current foreground window. Every
// implementation must be safe to call frequently (every couple of
// seconds) without noticeable CPU cost - see section 50's performance
// budget.
type ActiveWindowWatcher interface {
	ActiveWindow() (ActiveWindowInfo, error)
}

// NewActiveWindowWatcher is implemented once per OS build, exactly like
// NewStartupManager in startup.go.
func NewActiveWindowWatcher() ActiveWindowWatcher {
	return newActiveWindowWatcher()
}
