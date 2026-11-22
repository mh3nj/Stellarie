package services

import (
	"fmt"
	"os"

	"stellarei/internal/platform"
)

// StartupService exposes section 24's "Run on startup" toggle and its
// three-way "Startup behavior" (Normal / Minimized to Tray / Silent in
// Tray) to the frontend. The OS-specific registration itself lives in
// internal/platform, which this service is a thin, validating adapter
// over - consistent with every other service in this package.
type StartupService struct {
	mgr      platform.StartupManager
	settings *SettingsService
}

// NewStartupService constructs a StartupService bound to settings for
// persisting the user's choice, and to the current OS's StartupManager
// (resolved at compile time by the platform package's build tags).
func NewStartupService(settings *SettingsService) *StartupService {
	return &StartupService{
		mgr:      platform.NewStartupManager(),
		settings: settings,
	}
}

// IsEnabled reports whether STELLAREI is currently registered to launch at
// login, reading the OS's own record directly (not just our cached
// setting) so the UI can't drift out of sync if the user removed the
// registration by hand.
func (s *StartupService) IsEnabled() (bool, error) {
	return s.mgr.IsRegistered()
}

// SetEnabled turns launch-at-login on or off. When turning it on, behavior
// must be one of "normal", "tray", or "silent" (section 24) - "tray" and
// "silent" both launch with --start-hidden; the distinction between them
// (silent additionally suppressing a startup notification) is a Phase 15
// (Notifications) concern, not a Phase 1 one, so both currently behave
// identically. behavior is ignored when enabled is false.
func (s *StartupService) SetEnabled(enabled bool, behavior string) error {
	if !enabled {
		if err := s.mgr.Unregister(); err != nil {
			return fmt.Errorf("startup: unregistering: %w", err)
		}
		return s.settings.SetBool(KeyStartupEnabled, false)
	}

	switch behavior {
	case "normal", "tray", "silent":
	default:
		return fmt.Errorf("startup: invalid behavior %q (want normal, tray, or silent)", behavior)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("startup: resolving executable path: %w", err)
	}

	var args []string
	if behavior == "tray" || behavior == "silent" {
		args = []string{"--start-hidden"}
	}

	if err := s.mgr.Register(execPath, args); err != nil {
		return fmt.Errorf("startup: registering: %w", err)
	}
	if err := s.settings.SetBool(KeyStartupEnabled, true); err != nil {
		return err
	}
	return s.settings.SetString(KeyStartupBehavior, behavior)
}
