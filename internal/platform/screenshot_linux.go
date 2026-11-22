//go:build linux

package platform

import (
	"context"
	"os/exec"
	"time"
)

// linuxScreenCapturer shells out to whichever screenshot tool is
// actually installed, trying a short list of common ones in order
// unlike Windows (one stable GDI API) or macOS (one built-in
// `screencapture` command), Linux desktops have no single universal
// screenshot mechanism, so this checks a few common CLI tools rather
// than assuming any one is present. Mirrors idle_linux.go's own "shell
// out, degrade gracefully" approach, for the same CGO-free,
// X11-vs-Wayland-uncertain reasons documented there.
type linuxScreenCapturer struct{}

func newScreenCapturer() ScreenCapturer {
	return linuxScreenCapturer{}
}

// linuxScreenshotTools are tried in order; the first one found on PATH
// is used. Each entry is (binary name, args-before-the-output-path).
var linuxScreenshotTools = []struct {
	bin  string
	args []string
}{
	{bin: "scrot", args: nil},                            // scrot <path>
	{bin: "gnome-screenshot", args: []string{"-f"}},      // gnome-screenshot -f <path>
	{bin: "import", args: []string{"-window", "root"}},   // ImageMagick: import -window root <path>
	{bin: "spectacle", args: []string{"-b", "-n", "-o"}}, // KDE: spectacle -b -n -o <path>
}

func (linuxScreenCapturer) CaptureScreen(path string) error {
	for _, tool := range linuxScreenshotTools {
		if _, err := exec.LookPath(tool.bin); err != nil {
			continue // not installed - try the next one
		}
		args := append(append([]string{}, tool.args...), path)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := exec.CommandContext(ctx, tool.bin, args...).Run()
		cancel()
		if err == nil {
			return nil
		}
		// A found-but-failing tool (e.g. no display server) - keep
		// trying the rest of the list rather than giving up on the
		// first failure.
	}
	return ErrUnavailable
}
