//go:build darwin

package platform

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

type darwinIdleWatcher struct{}

func newIdleWatcher() IdleWatcher {
	return darwinIdleWatcher{}
}

// hidIdleTimePattern extracts the HIDIdleTime value (nanoseconds since
// last HID event) from `ioreg -c IOHIDSystem` output, e.g.
// `"HIDIdleTime" = 123456789`.
var hidIdleTimePattern = regexp.MustCompile(`"HIDIdleTime"\s*=\s*(\d+)`)

func (darwinIdleWatcher) IdleDuration() (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ioreg", "-c", "IOHIDSystem")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0, ErrUnavailable
	}

	match := hidIdleTimePattern.FindSubmatch(stdout.Bytes())
	if match == nil {
		return 0, ErrUnavailable
	}
	nanoseconds, err := strconv.ParseInt(string(match[1]), 10, 64)
	if err != nil {
		return 0, ErrUnavailable
	}
	return time.Duration(nanoseconds), nil
}
