//go:build !windows && !darwin && !linux

package alert

import "fmt"

// The three platforms STELLAREI ships on all have a real backend (see
// play_windows.go / play_darwin.go / play_linux.go). This stub exists so
// the package still builds for anything else (a BSD, a wasm target, a
// future GOOS) rather than failing the whole build for one missing
// sound. It reports honestly instead of pretending to succeed, so the
// failure shows up in the log rather than as a mystery silence.
func playFile(path string) error {
	return fmt.Errorf("no audio backend for this platform (tone written to %s)", path)
}
