//go:build darwin

package alert

import (
	"fmt"
	"os/exec"
)

// afplay ships with every macOS install (it is part of the base system,
// not Xcode tools) and needs no audio permission for a plain file
// playback, unlike anything routed through AVFoundation's microphone or
// speech paths. Command.Output (rather than Run) captures afplay's own
// stderr so a failure says *why* instead of just "exit status 1".
func playFile(path string) error {
	out, err := exec.Command("afplay", path).Output()
	if err != nil {
		return fmt.Errorf("afplay %q: %w (%s)", path, err, string(out))
	}
	return nil
}
