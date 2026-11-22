//go:build darwin

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// launchAgentLabel doubles as the plist filename stem and the bundle
// identifier LaunchAgents expect.
const launchAgentLabel = "com.stellarei.app"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>
`

type darwinStartupManager struct{}

func newStartupManager() StartupManager {
	return darwinStartupManager{}
}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func (darwinStartupManager) IsRegistered() (bool, error) {
	path, err := plistPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("startup(darwin): stat plist: %w", err)
	}
	return true, nil
}

func (darwinStartupManager) Register(execPath string, args []string) error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("startup(darwin): creating LaunchAgents dir: %w", err)
	}

	var argsXML strings.Builder
	fmt.Fprintf(&argsXML, "\t\t<string>%s</string>\n", execPath)
	for _, a := range args {
		fmt.Fprintf(&argsXML, "\t\t<string>%s</string>\n", a)
	}

	contents := fmt.Sprintf(plistTemplate, launchAgentLabel, argsXML.String())
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("startup(darwin): writing plist: %w", err)
	}

	// Note: a full implementation should also `launchctl load` the agent
	// so the change takes effect without a logout/login cycle. That means
	// shelling out to /bin/launchctl with a fixed, non-user-controlled
	// argument list (never built from unsanitized input) - left as a
	// follow-up since macOS is a secondary target (section 2).
	return nil
}

func (darwinStartupManager) Unregister() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("startup(darwin): removing plist: %w", err)
	}
	return nil
}
