// Package shortcuts implements the pure, testable half of spec section
// 45's global keyboard shortcuts: which actions exist, what their
// default accelerators are, whether a user-typed accelerator string is
// syntactically valid, and whether a proposed set of assignments
// collides with itself. The actual OS-level registration
// (app.GlobalShortcut.Register/Unregister) lives in main.go, since it
// needs the real Wails app instance and the actual action closures
// this package knows nothing about Wails, same "internal/ stays pure"
// convention as internal/closebehavior.
package shortcuts

import "strings"

// Action identifies one of STELLAREI's global shortcuts. String-backed
// (not iota) because these values are also used as Settings table keys
// and frontend action ids, so a renumbering must never change a
// persisted user's saved accelerator.
type Action string

const (
	ActionStartFocus    Action = "focus"
	ActionPauseTracking Action = "pause"
	ActionToggleTimer   Action = "timer"
	ActionIHaveNoIdea   Action = "no_idea"
)

// Actions lists every known action, in the fixed display order the
// Settings UI presents them in.
//
// There is deliberately no action for bringing the window to the front.
// The app opens by clicking its tray icon or its shortcut; a global
// key combo that could pop it up at any moment was more nuisance than
// convenience, so it was removed outright rather than left as a default
// someone had to discover in Settings and turn off.
var Actions = []Action{
	ActionStartFocus,
	ActionPauseTracking,
	ActionToggleTimer,
	ActionIHaveNoIdea,
}

// defaults are the accelerators main.go registers for every action on a
// fresh install, before any user customization exists.
var defaults = map[Action]string{
	ActionStartFocus:    "CmdOrCtrl+Shift+F",
	ActionPauseTracking: "CmdOrCtrl+Shift+P",
	ActionToggleTimer:   "CmdOrCtrl+Shift+T",
	ActionIHaveNoIdea:   "CmdOrCtrl+Shift+I",
}

// DefaultAccelerator returns action's built-in default, or "" if action
// isn't recognized.
func DefaultAccelerator(action Action) string {
	return defaults[action]
}

// IsKnownAction reports whether action is one of the registered actions.
func IsKnownAction(action Action) bool {
	_, ok := defaults[action]
	return ok
}

// recognized modifier tokens, matching Wails v3's own accelerator
// syntax (https://v3.wails.io/features/keyboard/global-shortcuts/,
// verified against the current release before this was written, same
// discipline as the original shortcut registration in main.go).
// Case-insensitive at the call site - normalize before comparing.
var validModifiers = map[string]bool{
	"cmdorctrl": true,
	"cmd":       true,
	"ctrl":      true,
	"control":   true,
	"alt":       true,
	"option":    true,
	"shift":     true,
	"super":     true,
	"meta":      true,
}

// Validate checks an accelerator string's SYNTAX only - that it's a
// "+"-joined sequence of one or more recognized modifiers followed by
// exactly one non-modifier key token (e.g. "CmdOrCtrl+Shift+S"). It
// cannot and does not check whether the OS or another application
// already owns that combination - only actually attempting
// registration (main.go, at Rebind time) can tell you that; Validate
// only rejects strings that could never register successfully
// regardless of what else is running.
func Validate(accelerator string) error {
	trimmed := strings.TrimSpace(accelerator)
	if trimmed == "" {
		return errEmptyAccelerator
	}
	parts := strings.Split(trimmed, "+")
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return errMalformedAccelerator
		}
	}
	if len(parts) < 2 {
		return errNoModifier
	}
	key := parts[len(parts)-1]
	if validModifiers[strings.ToLower(strings.TrimSpace(key))] {
		return errNoFinalKey
	}
	for _, m := range parts[:len(parts)-1] {
		if !validModifiers[strings.ToLower(strings.TrimSpace(m))] {
			return &unrecognizedModifierError{modifier: m}
		}
	}
	return nil
}

// Normalize lowercases and trims each "+"-separated token, so
// "cmdorctrl+shift+s" and "CmdOrCtrl + Shift + S" compare equal for
// conflict detection. It does not reorder tokens - "Shift+CmdOrCtrl+S"
// is deliberately treated as different from "CmdOrCtrl+Shift+S" (Wails/
// the OS may or may not treat those as equivalent; safer to require an
// exact match than to guess).
func Normalize(accelerator string) string {
	parts := strings.Split(accelerator, "+")
	for i, p := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(p))
	}
	return strings.Join(parts, "+")
}

// DetectConflicts returns, for every accelerator (normalized) that is
// assigned to more than one action in assignments, the list of actions
// sharing it. An assignments map with no collisions returns an empty
// map. This can only ever detect conflicts AMONG STELLAREI'S OWN five
// shortcuts - whether a given accelerator is already bound by the
// operating system or another application can only be discovered by
// actually attempting registration (main.go's job, at Rebind time),
// not by anything this pure function can know.
func DetectConflicts(assignments map[Action]string) map[string][]Action {
	byAccelerator := make(map[string][]Action)
	for action, accel := range assignments {
		norm := Normalize(accel)
		byAccelerator[norm] = append(byAccelerator[norm], action)
	}
	conflicts := make(map[string][]Action)
	for accel, actions := range byAccelerator {
		if len(actions) > 1 {
			conflicts[accel] = actions
		}
	}
	return conflicts
}
