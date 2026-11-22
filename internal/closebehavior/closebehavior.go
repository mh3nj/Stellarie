// Package closebehavior decides what an "X" click on the main window
// should do. It intentionally knows nothing about Wails, SQLite, or the
// tray - it's a pure mapping from a stored setting string to a Behavior,
// so the decision itself is trivially unit-testable and the framework
// wiring (in main.go, which registers the actual window-close hook) stays
// a thin adapter around it.
package closebehavior

// Behavior is the resolved action STELLAREI takes when the user closes the
// main window (section 73).
type Behavior string

const (
	// CloseApplication terminates the whole application, including the
	// background Activity Engine. Rarely the right default for a
	// continuously-running tracker, but it's the user's call.
	CloseApplication Behavior = "close"

	// MinimizeToTray hides the window; the process and all background
	// services (tracking, timers, alarms) keep running (section 73).
	MinimizeToTray Behavior = "tray"

	// AskEveryTime means the window-close hook should cancel the native
	// close event and show the in-app "Close STELLAREI / Minimize to
	// Tray / Cancel" prompt instead of resolving automatically.
	AskEveryTime Behavior = "ask"
)

// SettingKey is the settings table key this package's value is stored
// under. Defined here (not in app/services) because this is the
// authoritative source of truth for what the valid values mean.
const SettingKey = "close_behavior"

// DefaultBehavior matches the spec's stated default: STELLAREI is a
// background tracker first, so an accidental window close shouldn't kill
// tracking.
const DefaultBehavior = MinimizeToTray

// Parse converts a raw settings-table string into a Behavior, falling back
// to DefaultBehavior for anything unrecognised (empty value on first run,
// or a value from a future version this build doesn't know about) rather
// than erroring - a close button must always do *something* sensible.
func Parse(raw string) Behavior {
	switch Behavior(raw) {
	case CloseApplication, MinimizeToTray, AskEveryTime:
		return Behavior(raw)
	default:
		return DefaultBehavior
	}
}

// Valid reports whether raw is one of the three accepted values. The
// Settings UI should use this to validate before writing, rather than
// relying on Parse's silent fallback.
func Valid(raw string) bool {
	switch Behavior(raw) {
	case CloseApplication, MinimizeToTray, AskEveryTime:
		return true
	default:
		return false
	}
}
