// Package config resolves where STELLAREI stores its data on disk.
//
// Locations are computed once at startup and are user-overridable via the
// settings table (Settings > Data, section 54) - the functions here only
// supply the *defaults*. Nothing in this package touches the database; it
// only knows about paths.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppName is used to build the default per-OS data directory
// (e.g. "%AppData%\STELLAREI" on Windows) and must stay stable across
// releases - changing it silently orphans existing user data.
const AppName = "STELLAREI"

// AppVersion is shown on the Diagnostics page (spec sections 55/80).
// Not currently shown on About - that page is deliberately static and
// backend-free (see its own comment, "genuinely complete in Phase 1"),
// and pulling in a binding call just for a version string would break
// that. Value is "0.1.0", matching frontend/package.json's own
// "version" field by convention - there's no automated check enforcing
// that yet, so bump both by hand together.
const AppVersion = "0.1.0"

// Paths holds every on-disk location STELLAREI cares about. All fields are
// absolute paths. Callers should treat this as read-only after Load.
type Paths struct {
	Root        string // top-level app-data directory
	Database    string // Root/stellarei.db
	Migrations  string // embedded at build time; this is only the on-disk override dir, if any
	Screenshots string // Root/screenshots
	Backups     string // Root/backups
	Exports     string // Root/exports
	Logs        string // Root/logs
}

// Load resolves the default Paths for the current OS and ensures every
// directory in it exists (0700 - this is private, per-user data). It does
// not create the database file itself; internal/database does that.
func Load() (*Paths, error) {
	root, err := defaultRoot()
	if err != nil {
		return nil, fmt.Errorf("config: resolving app data root: %w", err)
	}

	p := &Paths{
		Root:        root,
		Database:    filepath.Join(root, "stellarei.db"),
		Screenshots: filepath.Join(root, "screenshots"),
		Backups:     filepath.Join(root, "backups"),
		Exports:     filepath.Join(root, "exports"),
		Logs:        filepath.Join(root, "logs"),
	}

	for _, dir := range []string{p.Root, p.Screenshots, p.Backups, p.Exports, p.Logs} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("config: creating %s: %w", dir, err)
		}
	}

	return p, nil
}

// defaultRoot returns the per-user application data directory.
//
//   - Windows: %AppData%\STELLAREI              (os.UserConfigDir -> AppData\Roaming)
//   - macOS:   ~/Library/Application Support/STELLAREI
//   - Linux:   $XDG_CONFIG_HOME/STELLAREI or ~/.config/STELLAREI
//
// os.UserConfigDir() already implements exactly this split across the three
// target OSes, so there is no need for platform-specific files here.
func defaultRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}
