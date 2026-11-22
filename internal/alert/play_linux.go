//go:build linux

package alert

import (
	"fmt"
	"os/exec"
	"strings"
)

// Linux has no single audio playback API that is present on every
// install (there is no guaranteed ALSA, PulseAudio or PipeWire, and
// calling into any of them directly would mean picking a sound server
// this app has no business depending on). So the tone is played through
// whichever standard player is actually installed, in order of how
// likely it is to exist on a modern desktop: PipeWire, then
// PulseAudio, then bare ALSA, then the freedesktop event-sound player,
// then the two media players that are commonly already present.
//
// A machine with none of them gets a logged failure, not a crash - the
// visible channels (toast + in-app banner) still fire.
var linuxPlayers = [][]string{
	{"pw-play"},
	{"paplay"},
	{"aplay", "-q"},
	{"canberra-gtk-play", "-f"},
	{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
	{"mpv", "--really-quiet", "--no-video"},
}

func playFile(path string) error {
	for _, candidate := range linuxPlayers {
		binary, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		args := append(append([]string{}, candidate[1:]...), path)
		if err := exec.Command(binary, args...).Run(); err != nil {
			return fmt.Errorf("%s %q: %w", candidate[0], path, err)
		}
		return nil
	}
	return fmt.Errorf("no supported audio player found (looked for %s)", strings.Join(linuxPlayerNames(), ", "))
}

// linuxPlayerNames lists the players linuxPlayers searches for, so the
// one-and-only error message names exactly what was tried instead of a
// hand-maintained second list that can drift out of sync.
func linuxPlayerNames() []string {
	names := make([]string, 0, len(linuxPlayers))
	for _, candidate := range linuxPlayers {
		names = append(names, candidate[0])
	}
	return names
}
