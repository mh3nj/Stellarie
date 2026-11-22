// Package platform contains the thin, OS-specific slivers of STELLAREI:
// run-on-startup registration today: idle detection and active-window
// detection land here too once Phase 2 (Activity Engine) starts.
//
// The rule for this package: interfaces here, implementations in
// *_windows.go / *_darwin.go / *_linux.go. Nothing outside this package
// should ever need a build tag or a runtime.GOOS switch - see section 2.
package platform

// StartupManager registers or removes STELLAREI from the OS's per-user
// "launch at login" mechanism. It never requires administrator/root
// privileges and never installs a system service (section 74) - STELLAREI
// is a normal user-level application.
type StartupManager interface {
	// IsRegistered reports whether STELLAREI is currently set to launch at
	// login, by reading back whatever Register wrote.
	IsRegistered() (bool, error)

	// Register enables launch-at-login, pointing at execPath (the absolute
	// path to the running executable) with the given extra arguments (e.g.
	// []string{"--start-hidden"} so a startup launch honours the user's
	// configured StartupBehavior - Normal / MinimizedToTray / SilentInTray
	// - without STELLAREI needing OS-specific code outside this package).
	Register(execPath string, args []string) error

	// Unregister disables launch-at-login. Calling it when nothing is
	// registered is not an error.
	Unregister() error
}

// NewStartupManager is implemented once per OS build (startup_windows.go,
// startup_darwin.go, startup_linux.go). Exactly one of those files is
// compiled into any given binary, selected by its `//go:build` tag, so
// there is no runtime branching here.
func NewStartupManager() StartupManager {
	return newStartupManager()
}
