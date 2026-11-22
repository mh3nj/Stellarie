// Package theme determines whether a set of theme tokens reads as "light"
// or "dark", from the tokens' own background color - not from the OS's
// dark-mode setting. That distinction matters: a user can pick a dark
// custom theme while their OS is in light mode (or vice versa), and the
// tray icon / in-app logo should follow the theme they chose in
// STELLAREI, not the OS's unrelated preference.
package theme

import (
	"fmt"
	"strconv"
	"strings"
)

// Mode is which logo/icon variant a theme calls for.
type Mode string

const (
	Dark  Mode = "dark"
	Light Mode = "light"
)

// ComputeMode classifies a theme by its background color's perceived
// luminance (the standard broadcast-luma weighting: 0.299R+0.587G+0.114B).
// backgroundHex accepts "#rrggbb" or "#rgb" (case-insensitive, leading #
// optional). A luminance below 0.5 reads as Dark.
//
// This is deliberately the same formula and threshold as
// frontend/src/themes/themes.ts's computeThemeMode - keep them in sync if
// either changes, so the Go-side tray icon and the frontend's logo/CSS
// never disagree about the same theme's mode.
func ComputeMode(backgroundHex string) (Mode, error) {
	r, g, b, err := parseHexColor(backgroundHex)
	if err != nil {
		return Dark, fmt.Errorf("theme: computing mode: %w", err)
	}
	luminance := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255.0
	if luminance < 0.5 {
		return Dark, nil
	}
	return Light, nil
}

func parseHexColor(s string) (r, g, b uint8, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")

	expand := func(c byte) string { return string([]byte{c, c}) }

	switch len(s) {
	case 3: // "rgb" shorthand -> "rrggbb"
		s = expand(s[0]) + expand(s[1]) + expand(s[2])
	case 6:
		// already full length
	default:
		return 0, 0, 0, fmt.Errorf("unrecognized color format %q (want #rgb or #rrggbb)", s)
	}

	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parsing hex color %q: %w", s, err)
	}
	return uint8(val >> 16), uint8(val >> 8), uint8(val), nil
}
