package platform

import "time"

// IdleWatcher reports how long it's been since the last keyboard/mouse
// input. Implementations must never read *what* the input was - only
// its timestamp - per the hard "no keylogging" rule in section 53.
type IdleWatcher interface {
	// IdleDuration returns time elapsed since the last user input.
	// A zero duration means the user is actively interacting right now.
	IdleDuration() (time.Duration, error)
}

// NewIdleWatcher is implemented once per OS build (idle_windows.go,
// idle_darwin.go, idle_linux.go).
func NewIdleWatcher() IdleWatcher {
	return newIdleWatcher()
}
