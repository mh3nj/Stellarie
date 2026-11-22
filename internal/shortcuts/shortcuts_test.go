package shortcuts

import (
	"errors"
	"testing"
)

func TestDefaultAccelerator_KnownAction(t *testing.T) {
	got := DefaultAccelerator(ActionStartFocus)
	if got != "CmdOrCtrl+Shift+F" {
		t.Errorf("DefaultAccelerator(focus) = %q, want %q", got, "CmdOrCtrl+Shift+F")
	}
}

func TestNoShowActionExists(t *testing.T) {
	// The global "bring the app to the front" shortcut was removed on
	// request: the app is opened from its tray icon or its shortcut, not
	// from a system-wide key combo. Guard against it quietly reappearing.
	if IsKnownAction(Action("show")) {
		t.Error(`IsKnownAction("show") = true, want false: the show/open shortcut was removed`)
	}
	for _, action := range Actions {
		if action == Action("show") {
			t.Error("Actions still contains the removed show action")
		}
	}
}

func TestDefaultAccelerator_UnknownActionReturnsEmpty(t *testing.T) {
	got := DefaultAccelerator(Action("bogus"))
	if got != "" {
		t.Errorf("DefaultAccelerator(bogus) = %q, want empty string", got)
	}
}

func TestIsKnownAction(t *testing.T) {
	if !IsKnownAction(ActionStartFocus) {
		t.Error("IsKnownAction(focus) = false, want true")
	}
	if IsKnownAction(Action("bogus")) {
		t.Error("IsKnownAction(bogus) = true, want false")
	}
}

func TestAllDefaultsAreValid(t *testing.T) {
	// Every registered action must carry a syntactically valid default,
	// so a fresh install's initial registration pass can never fail.
	for _, action := range Actions {
		accel := DefaultAccelerator(action)
		if err := Validate(accel); err != nil {
			t.Errorf("Validate(default for %s) = %v, want nil (default: %q)", action, err, accel)
		}
	}
}

func TestValidate_RejectsEmpty(t *testing.T) {
	if err := Validate(""); err == nil {
		t.Error("Validate(\"\") = nil, want an error")
	}
	if err := Validate("   "); err == nil {
		t.Error("Validate(whitespace only) = nil, want an error")
	}
}

func TestValidate_RejectsNoModifier(t *testing.T) {
	if err := Validate("S"); err == nil {
		t.Error("Validate(\"S\") = nil, want an error (a bare key with no modifier)")
	}
}

func TestValidate_RejectsModifierOnly(t *testing.T) {
	if err := Validate("CmdOrCtrl+Shift"); err == nil {
		t.Error("Validate(\"CmdOrCtrl+Shift\") = nil, want an error (no actual key, just modifiers)")
	}
}

func TestValidate_RejectsUnrecognizedModifier(t *testing.T) {
	err := Validate("Comand+Shift+S")
	if err == nil {
		t.Fatal("Validate(\"Comand+Shift+S\") = nil, want an error (typo'd modifier)")
	}
	var unrecognized *unrecognizedModifierError
	if !errors.As(err, &unrecognized) {
		t.Errorf("error type = %T, want *unrecognizedModifierError", err)
	}
}

func TestValidate_RejectsMalformedDoublePlus(t *testing.T) {
	if err := Validate("CmdOrCtrl++S"); err == nil {
		t.Error("Validate(\"CmdOrCtrl++S\") = nil, want an error (empty segment)")
	}
	if err := Validate("CmdOrCtrl+Shift+"); err == nil {
		t.Error("Validate(\"CmdOrCtrl+Shift+\") = nil, want an error (trailing +)")
	}
}

func TestValidate_AcceptsCaseInsensitiveModifiers(t *testing.T) {
	if err := Validate("cmdorctrl+shift+S"); err != nil {
		t.Errorf("Validate(\"cmdorctrl+shift+S\") = %v, want nil (modifiers are case-insensitive)", err)
	}
}

func TestValidate_AcceptsMultipleRealWorldExamples(t *testing.T) {
	valid := []string{"CmdOrCtrl+Shift+S", "Alt+F", "Ctrl+Alt+Delete", "Super+Space"}
	for _, accel := range valid {
		if err := Validate(accel); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", accel, err)
		}
	}
}

func TestNormalize_CaseAndWhitespaceInsensitive(t *testing.T) {
	a := Normalize("CmdOrCtrl+Shift+S")
	b := Normalize("cmdorctrl + shift + s")
	if a != b {
		t.Errorf("Normalize disagreement: %q vs %q, want equal", a, b)
	}
}

func TestNormalize_PreservesTokenOrder(t *testing.T) {
	a := Normalize("Shift+CmdOrCtrl+S")
	b := Normalize("CmdOrCtrl+Shift+S")
	if a == b {
		t.Error("Normalize treated different token orders as equal - order should be preserved, not sorted")
	}
}

func TestDetectConflicts_NoOverlapReturnsEmpty(t *testing.T) {
	assignments := map[Action]string{
		ActionStartFocus:    "CmdOrCtrl+Shift+F",
		ActionPauseTracking: "CmdOrCtrl+Shift+P",
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 0 {
		t.Errorf("DetectConflicts() = %v, want empty map", conflicts)
	}
}

func TestDetectConflicts_FindsExactDuplicate(t *testing.T) {
	assignments := map[Action]string{
		ActionStartFocus:    "CmdOrCtrl+Shift+S",
		ActionPauseTracking: "CmdOrCtrl+Shift+S",
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 1 {
		t.Fatalf("DetectConflicts() found %d conflicting accelerators, want 1: %v", len(conflicts), conflicts)
	}
	actions := conflicts[Normalize("CmdOrCtrl+Shift+S")]
	if len(actions) != 2 {
		t.Errorf("conflicting actions = %v, want both ActionStartFocus and ActionPauseTracking", actions)
	}
}

func TestDetectConflicts_FindsDuplicateDespiteCaseAndSpacing(t *testing.T) {
	assignments := map[Action]string{
		ActionStartFocus:    "CmdOrCtrl+Shift+S",
		ActionPauseTracking: "cmdorctrl + shift + s",
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 1 {
		t.Errorf("DetectConflicts() found %d conflicts, want 1 (same accelerator, different case/spacing)", len(conflicts))
	}
}

func TestDetectConflicts_AllDefaultsAreConflictFree(t *testing.T) {
	// The built-in defaults are all DIFFERENT accelerators by
	// construction, so confirm that still holds: a future edit that
	// accidentally reuses one would be a real regression this catches.
	assignments := make(map[Action]string, len(Actions))
	for _, action := range Actions {
		assignments[action] = DefaultAccelerator(action)
	}
	conflicts := DetectConflicts(assignments)
	if len(conflicts) != 0 {
		t.Errorf("DetectConflicts(defaults) = %v, want empty: the built-in defaults must never collide with each other", conflicts)
	}
}
