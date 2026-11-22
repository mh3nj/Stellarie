package shortcuts

import (
	"errors"
	"fmt"
)

var (
	errEmptyAccelerator     = errors.New("shortcuts: accelerator must not be empty")
	errMalformedAccelerator = errors.New(`shortcuts: accelerator has an empty segment (check for stray "+" or spaces)`)
	errNoModifier           = errors.New("shortcuts: accelerator needs at least one modifier (e.g. CmdOrCtrl+Shift+S, not just S)")
	errNoFinalKey           = errors.New("shortcuts: accelerator's last segment must be the key, not another modifier")
)

// unrecognizedModifierError names the specific unrecognized token
// rather than a generic message, so a typo like "Cmdorctrl" (wrong
// case is fine - Validate lowercases before checking - but
// "Comand+Shift+S" isn't) is easy to spot and fix.
type unrecognizedModifierError struct {
	modifier string
}

func (e *unrecognizedModifierError) Error() string {
	return fmt.Sprintf("shortcuts: %q is not a recognized modifier (expected CmdOrCtrl, Cmd, Ctrl, Alt, Option, Shift, Super, or Meta)", e.modifier)
}
