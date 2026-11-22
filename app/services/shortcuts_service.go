package services

import (
	"fmt"

	"stellarei/internal/shortcuts"
)

// ShortcutRegistrar is the live, OS-touching half of this feature
// implemented in main.go, which is the only place with the real Wails
// app instance and the actual action closures (Show STELLAREI, Start
// Focus, etc.) a shortcut needs to invoke. Kept as an interface here so
// this package (and its tests, if this struct ever grows enough logic
// to warrant them beyond what internal/shortcuts already covers) stays
// free of a direct github.com/wailsapp/wails/v3 import - same
// import-boundary reasoning as notify.Dispatcher and every Resolver
// interface in internal/activity.
type ShortcutRegistrar interface {
	// Rebind unregisters action's currently-registered accelerator (if
	// any) and registers newAccelerator in its place, using the SAME
	// action closure main.go originally registered it with. Returns
	// whatever real error the OS/Wails registration call itself
	// produces - most commonly, the accelerator is already claimed by
	// another application. This is the only way a real, external
	// conflict can ever be detected (see internal/shortcuts.
	// DetectConflicts' own doc comment on why it can only catch
	// conflicts among STELLAREI's own shortcuts, not this kind).
	Rebind(action shortcuts.Action, newAccelerator string) error
}

// ShortcutRegistrarFunc adapts a bare func to ShortcutRegistrar - same
// pattern as http.HandlerFunc. main.go's registrar is naturally a
// closure (it needs to close over the real *application.App and the
// action-closures map, neither of which this package should know
// about), not a type worth giving its own named struct in main.go.
type ShortcutRegistrarFunc func(action shortcuts.Action, newAccelerator string) error

func (f ShortcutRegistrarFunc) Rebind(action shortcuts.Action, newAccelerator string) error {
	return f(action, newAccelerator)
}

// ShortcutSettingKey is exported so main.go's initial-registration loop
// (which runs once at startup, before any frontend request exists to
// go through ShortcutsService itself) reads the exact same Settings key
// this service reads and writes - the two can never drift apart into
// looking at different keys for the same action.
func ShortcutSettingKey(action shortcuts.Action) string {
	return settingKeyFor(action)
}

// ShortcutsService is bound to the frontend via application.NewService
// in main.go.
type ShortcutsService struct {
	settings  *SettingsService
	registrar ShortcutRegistrar
}

func NewShortcutsService(settings *SettingsService, registrar ShortcutRegistrar) *ShortcutsService {
	return &ShortcutsService{settings: settings, registrar: registrar}
}

func settingKeyFor(action shortcuts.Action) string {
	return keyShortcutPrefix + string(action)
}

// ShortcutView is what the frontend renders one row from.
type ShortcutView struct {
	Action      string `json:"action"`
	Accelerator string `json:"accelerator"`
	IsCustom    bool   `json:"isCustom"`
}

// currentAssignments reads every action's effective accelerator
// its stored override if one exists, else its built-in default - in
// one pass. Used both by ListShortcuts and, internally, by SetShortcut
// to build the full five-way map DetectConflicts needs (a single
// action's own new value can only be checked for conflicts against
// what every OTHER action currently has).
func (s *ShortcutsService) currentAssignments() (map[shortcuts.Action]string, map[shortcuts.Action]bool, error) {
	assignments := make(map[shortcuts.Action]string, len(shortcuts.Actions))
	isCustom := make(map[shortcuts.Action]bool, len(shortcuts.Actions))
	for _, action := range shortcuts.Actions {
		stored, err := s.settings.GetString(settingKeyFor(action))
		if err != nil {
			return nil, nil, fmt.Errorf("shortcuts: reading %s: %w", action, err)
		}
		if stored == "" {
			assignments[action] = shortcuts.DefaultAccelerator(action)
			isCustom[action] = false
		} else {
			assignments[action] = stored
			isCustom[action] = true
		}
	}
	return assignments, isCustom, nil
}

func (s *ShortcutsService) ListShortcuts() ([]ShortcutView, error) {
	assignments, isCustom, err := s.currentAssignments()
	if err != nil {
		return nil, err
	}
	out := make([]ShortcutView, 0, len(shortcuts.Actions))
	for _, action := range shortcuts.Actions {
		out = append(out, ShortcutView{
			Action:      string(action),
			Accelerator: assignments[action],
			IsCustom:    isCustom[action],
		})
	}
	return out, nil
}

// SetShortcut validates, checks for a conflict against STELLAREI's own
// other four shortcuts, attempts the LIVE re-registration, and only
// persists the change once that live attempt actually succeeds - so a
// rejected accelerator (malformed, colliding with another of
// STELLAREI's own shortcuts, or genuinely already owned by another
// application at the OS level) never leaves a broken setting saved.
func (s *ShortcutsService) SetShortcut(action, accelerator string) error {
	act := shortcuts.Action(action)
	if !shortcuts.IsKnownAction(act) {
		return fmt.Errorf("shortcuts: %q is not a recognized action", action)
	}
	if err := shortcuts.Validate(accelerator); err != nil {
		return err
	}

	assignments, _, err := s.currentAssignments()
	if err != nil {
		return err
	}
	assignments[act] = accelerator
	if conflicts := shortcuts.DetectConflicts(assignments); len(conflicts) > 0 {
		for _, actions := range conflicts {
			if containsAction(actions, act) {
				other := otherAction(actions, act)
				return fmt.Errorf("shortcuts: %q is already assigned to %s - choose a different combination", accelerator, other)
			}
		}
	}

	if err := s.registrar.Rebind(act, accelerator); err != nil {
		return fmt.Errorf("shortcuts: couldn't register %q (likely already in use by another application): %w", accelerator, err)
	}
	return s.settings.SetString(settingKeyFor(act), accelerator)
}

// ResetShortcut restores action to its built-in default, through the
// same validate-rebind-then-persist path as SetShortcut (a reset can
// still conflict, in principle, if some OTHER action was already
// custom-assigned to this action's default accelerator).
func (s *ShortcutsService) ResetShortcut(action string) error {
	act := shortcuts.Action(action)
	if !shortcuts.IsKnownAction(act) {
		return fmt.Errorf("shortcuts: %q is not a recognized action", action)
	}
	def := shortcuts.DefaultAccelerator(act)
	if err := s.registrar.Rebind(act, def); err != nil {
		return fmt.Errorf("shortcuts: couldn't restore default %q: %w", def, err)
	}
	// Empty string, not def - see currentAssignments: unset means
	// "follow the default," which is what a reset actually means.
	return s.settings.SetString(settingKeyFor(act), "")
}

func containsAction(actions []shortcuts.Action, target shortcuts.Action) bool {
	for _, a := range actions {
		if a == target {
			return true
		}
	}
	return false
}

// otherAction returns the first action in actions that isn't target
// used only for the error message ("already assigned to <other>"),
// where actions is always exactly the 2-element conflict pair target
// itself belongs to.
func otherAction(actions []shortcuts.Action, target shortcuts.Action) shortcuts.Action {
	for _, a := range actions {
		if a != target {
			return a
		}
	}
	return target
}
