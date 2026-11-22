//go:build linux

package platform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// linuxActiveWindowWatcher shells out to xdotool rather than linking an
// X11 client library - this keeps the whole project CGO-free (see
// internal/database/database.go's comment on the same tradeoff) at the
// cost of only working under X11/XWayland with xdotool installed. Linux
// is an explicitly secondary target (spec section 2); Windows gets the
// real Win32 implementation in activewindow_windows.go.
type linuxActiveWindowWatcher struct{}

func newActiveWindowWatcher() ActiveWindowWatcher {
	return linuxActiveWindowWatcher{}
}

func (linuxActiveWindowWatcher) ActiveWindow() (ActiveWindowInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Every argv element below is a fixed literal - nothing here is ever
	// built from window titles, file paths, or other untrusted input, so
	// there's no command-injection surface (section 53).
	pidOut, err := runXdotool(ctx, "getactivewindow", "getwindowpid")
	if err != nil {
		return ActiveWindowInfo{}, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidOut))
	if err != nil {
		return ActiveWindowInfo{}, fmt.Errorf("platform(linux): parsing pid %q: %w", pidOut, err)
	}

	titleOut, err := runXdotool(ctx, "getactivewindow", "getwindowname")
	if err != nil {
		// A window with no title is legitimate; keep the PID/process we
		// already have rather than discarding the whole sample.
		titleOut = ""
	}

	processName, err := processNameFromPID(pid)
	if err != nil {
		return ActiveWindowInfo{}, err
	}

	return ActiveWindowInfo{
		ProcessName: processName,
		WindowTitle: strings.TrimSpace(titleOut),
		PID:         pid,
	}, nil
}

func runXdotool(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "xdotool", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// Covers "xdotool not installed" and "no X11 display" (Wayland
		// without XWayland, or a genuinely headless session) uniformly
		// both mean the same thing to a caller: no sample this tick.
		return "", ErrUnavailable
	}
	return stdout.String(), nil
}

// processNameFromPID reads /proc/<pid>/comm directly, which the kernel
// truncates to 15 bytes - acceptable here since it's used for
// categorization matching against user-configured process names, the
// same length limit most Linux process managers live with.
func processNameFromPID(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", ErrUnavailable
	}
	return strings.TrimSpace(string(data)), nil
}
