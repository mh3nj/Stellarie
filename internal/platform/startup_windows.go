//go:build windows

package platform

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath is the standard per-user autostart location. Writing here
// requires no elevation - this is intentional; STELLAREI must never prompt
// for admin rights just to toggle "run on startup" (section 74).
const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "STELLAREI"
)

type windowsStartupManager struct{}

func newStartupManager() StartupManager {
	return windowsStartupManager{}
}

func (windowsStartupManager) IsRegistered() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("startup(windows): opening Run key: %w", err)
	}
	defer key.Close()

	_, _, err = key.GetStringValue(valueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("startup(windows): reading value: %w", err)
	}
	return true, nil
}

func (windowsStartupManager) Register(execPath string, args []string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("startup(windows): opening Run key for write: %w", err)
	}
	defer key.Close()

	// Quote the executable path defensively (paths under "Program Files"
	// contain spaces) and append any launch arguments as-is - these come
	// from our own code (see startup.go doc comment), never from
	// unsanitized user input, so no further escaping is required here.
	cmd := fmt.Sprintf(`"%s"`, execPath)
	if len(args) > 0 {
		cmd = cmd + " " + strings.Join(args, " ")
	}

	if err := key.SetStringValue(valueName, cmd); err != nil {
		return fmt.Errorf("startup(windows): writing value: %w", err)
	}
	return nil
}

func (windowsStartupManager) Unregister() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("startup(windows): opening Run key for delete: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("startup(windows): deleting value: %w", err)
	}
	return nil
}
