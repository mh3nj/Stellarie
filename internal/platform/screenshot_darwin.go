//go:build darwin

package platform

import (
	"context"
	"os/exec"
	"time"
)

// darwinScreenCapturer shells out to `screencapture`, macOS's own
// built-in, stable command-line screenshot tool (present on every macOS
// install, no third-party dependency needed) - the same
// shell-a-stable-OS-tool approach idle_darwin.go already uses for
// `ioreg`.
type darwinScreenCapturer struct{}

func newScreenCapturer() ScreenCapturer {
	return darwinScreenCapturer{}
}

func (darwinScreenCapturer) CaptureScreen(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// -x: no camera-shutter sound, no visible flash - a background
	// screenshot tool interrupting the person with a sound effect every
	// interval would defeat the point of it being unobtrusive.
	if err := exec.CommandContext(ctx, "screencapture", "-x", path).Run(); err != nil {
		return ErrUnavailable
	}
	return nil
}
