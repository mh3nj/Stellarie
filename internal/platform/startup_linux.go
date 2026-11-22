//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const desktopEntryTemplate = `[Desktop Entry]
Type=Application
Name=STELLAREI
Exec=%s
X-GNOME-Autostart-enabled=true
NoDisplay=false
Terminal=false
`

type linuxStartupManager struct{}

func newStartupManager() StartupManager {
	return linuxStartupManager{}
}

func autostartPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "stellarei.desktop"), nil
}

func (linuxStartupManager) IsRegistered() (bool, error) {
	path, err := autostartPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("startup(linux): stat desktop entry: %w", err)
	}
	return true, nil
}

func (linuxStartupManager) Register(execPath string, args []string) error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("startup(linux): creating autostart dir: %w", err)
	}

	execLine := execPath
	if len(args) > 0 {
		execLine = execPath + " " + strings.Join(args, " ")
	}

	contents := fmt.Sprintf(desktopEntryTemplate, execLine)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("startup(linux): writing desktop entry: %w", err)
	}
	return nil
}

func (linuxStartupManager) Unregister() error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("startup(linux): removing desktop entry: %w", err)
	}
	return nil
}
