//go:build linux

package platform

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type linuxIdleWatcher struct{}

func newIdleWatcher() IdleWatcher {
	return linuxIdleWatcher{}
}

func (linuxIdleWatcher) IdleDuration() (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xprintidle")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// xprintidle not installed, or no X11 display (Wayland/headless)
		// same "no answer this tick" contract as the active-window watcher.
		return 0, ErrUnavailable
	}

	ms, err := strconv.ParseInt(strings.TrimSpace(stdout.String()), 10, 64)
	if err != nil {
		return 0, ErrUnavailable
	}
	return time.Duration(ms) * time.Millisecond, nil
}
