//go:build darwin

package platform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// darwinActiveWindowWatcher uses AppleScript via osascript rather than
// linking Cocoa/ApplicationServices through cgo - same CGO-free rationale
// as the Linux xdotool implementation. macOS is a secondary target
// (spec section 2); this will prompt for Accessibility/Automation
// permission on first run, which is expected and unavoidable for any
// active-window reader on macOS, native or not.
type darwinActiveWindowWatcher struct{}

func newActiveWindowWatcher() ActiveWindowWatcher {
	return darwinActiveWindowWatcher{}
}

const frontAppScript = `
tell application "System Events"
	set frontApp to first application process whose frontmost is true
	set appName to name of frontApp
	set appPID to unix id of frontApp
	set winTitle to ""
	try
		set winTitle to name of first window of frontApp
	end try
	return appName & "|||" & appPID & "|||" & winTitle
end tell
`

func (darwinActiveWindowWatcher) ActiveWindow() (ActiveWindowInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "osascript", "-e", frontAppScript)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// Most commonly: Automation permission for "System Events" hasn't
		// been granted yet. Treat as "no sample this tick" rather than a
		// hard failure - section 56.
		return ActiveWindowInfo{}, ErrUnavailable
	}

	parts := strings.SplitN(strings.TrimSpace(stdout.String()), "|||", 3)
	if len(parts) != 3 {
		return ActiveWindowInfo{}, fmt.Errorf("platform(darwin): unexpected osascript output %q", stdout.String())
	}

	pid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return ActiveWindowInfo{}, fmt.Errorf("platform(darwin): parsing pid: %w", err)
	}

	return ActiveWindowInfo{
		ProcessName: parts[0],
		WindowTitle: parts[2],
		PID:         pid,
	}, nil
}
