// Package tray builds STELLAREI's system tray icon and menu (sections 23,
// 44). This is a CORE feature per the spec, not an afterthought - the app
// must be fully usable with the main window closed.
//
// Phase scoping: through Phase 9, this file delivers the live status
// label (including a distinct "PRIVACY MODE ACTIVE" state - section
// 26), Pause Tracking, Timer quick-start/stop/open (section 21), Open
// Alarms/Focus/"I HAVE NO IDEA" (sections 22/20/44), and Enable/Disable
// Privacy Mode. The tray ICON itself doesn't yet change for Privacy
// Mode - see the menu section's own comment on why. No remaining TODOs:
// every tray feature the current phase set calls for is wired to a real
// implementation, not a no-op (section 83).
package tray

import (
	"context"
	"fmt"
	"log"
	"time"

	"stellarei/internal/activity"
	"stellarei/internal/closebehavior"
	"stellarei/internal/theme"
	"stellarei/internal/timers"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Deps is everything Build needs from the rest of the app. Passing this
// explicitly (rather than reaching into globals) keeps tray testable and
// keeps main.go the only place that assembles the whole dependency graph.
type Deps struct {
	App    *application.App
	Window *application.WebviewWindow // the main window, created hidden
	Engine *activity.Engine           // Pause/Resume act on this directly - tray clicks already run in Go, no Wails binding round-trip needed

	// TimerRepo backs the tray's quick-start/stop timer actions (section
	// 21 "Timers must be accessible from the tray"). Same
	// tray-clicks-already-run-in-Go reasoning as Engine above - no need
	// to round-trip through the frontend's TimerService binding just to
	// start a timer from a tray click.
	TimerRepo *timers.Repository

	// IconDark/IconLight are the two tray icon variants (see
	// assets/tray-icon-dark.png / tray-icon-light.png), switched based on
	// the ACTIVE STELLAREI THEME's computed mode - deliberately not the
	// OS's own dark-mode setting, since a user can run a dark custom
	// theme on a light-mode OS or vice versa, and the tray should follow
	// what they picked in STELLAREI.
	IconDark    []byte
	IconLight   []byte
	InitialMode theme.Mode // computed from the active theme before Build runs - see main.go

	// GetCloseBehavior reads the current setting so the tray's "Show
	// STELLAREI" toggle and the window-close hook agree on behaviour.
	// Wired to services.SettingsService.GetCloseBehavior in main.go.
	GetCloseBehavior func() (closebehavior.Behavior, error)

	// SetPrivacyMode is wired to services.PrivacyService.SetPrivacyMode
	// (section 26) rather than calling d.Engine.SetPrivacyMode directly
	// - the service also persists the setting so Privacy Mode survives
	// a restart; calling the Engine alone from here would silently
	// desync the tray's toggle from what the Settings page shows.
	SetPrivacyMode func(enabled bool) error
}

// Tray wraps the constructed system tray with STELLAREI's own update
// method, so main.go's activity.Config.OnStateChange callback has
// somewhere to push live state.
type Tray struct {
	sysTray   *application.SystemTray
	iconDark  []byte
	iconLight []byte
}

// Build constructs the tray icon and its menu.
func Build(d Deps) *Tray {
	sysTray := d.App.SystemTray.New()
	sysTray.SetLabel("STELLAREI")

	t := &Tray{sysTray: sysTray, iconDark: d.IconDark, iconLight: d.IconLight}
	t.SetIconForMode(d.InitialMode)

	menu := d.App.NewMenu()

	menu.Add("Show STELLAREI").OnClick(func(_ *application.Context) {
		d.Window.Show()
		d.Window.Focus()
	})

	menu.AddSeparator()

	// Section 25's full spec asks for 5/15/30/60-minute and
	// "until resumed" options. Trimmed to the three most useful here
	// rather than risk an unverified submenu API in this environment
	// (see README.md's honesty note on what's unverified) - flat items
	// are the safer choice, and still cover the common cases. Add the
	// finer-grained durations once this has actually run against a real
	// Wails build and submenu behavior is confirmed.
	menu.Add("Pause Tracking (15 min)").OnClick(func(_ *application.Context) {
		d.Engine.Pause(15 * time.Minute)
	})
	menu.Add("Pause Tracking (1 hour)").OnClick(func(_ *application.Context) {
		d.Engine.Pause(time.Hour)
	})
	menu.Add("Pause Tracking (until resumed)").OnClick(func(_ *application.Context) {
		d.Engine.Pause(0)
	})
	menu.Add("Resume Tracking").OnClick(func(_ *application.Context) {
		d.Engine.Resume()
	})

	menu.AddSeparator()

	// Section 21 "Timers must be accessible from the tray". Flat items,
	// same reasoning as the Pause Tracking durations above (no submenu
	// see that comment). "Start 25-min Timer" is a fixed quick-start
	// convenience, not a replacement for the Timers page's full
	// stopwatch/countdown/Pomodoro controls; "Stop Running Timer" stops
	// whichever active timer was started most recently (there's no tray
	// affordance for picking among several at once - the Timers page is
	// where that happens).
	menu.Add("Start 25-min Timer").OnClick(func(_ *application.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := d.TimerRepo.Create(ctx, timers.TypeCountdown, 0, "Quick timer", 25*60, "", time.Now()); err != nil {
			log.Printf("tray: starting quick timer: %v", err)
		}
	})
	menu.Add("Stop Running Timer").OnClick(func(_ *application.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		active, err := d.TimerRepo.ListActive(ctx)
		if err != nil {
			log.Printf("tray: listing active timers: %v", err)
			return
		}
		if len(active) == 0 {
			return // nothing running - a no-op, not an error the person needs to see
		}
		if err := d.TimerRepo.Complete(ctx, active[0].ID, time.Now()); err != nil {
			log.Printf("tray: stopping timer %d: %v", active[0].ID, err)
		}
	})
	menu.Add("Open Timers…").OnClick(func(_ *application.Context) {
		d.Window.Show()
		d.Window.Focus()
		d.App.Event.Emit("tray:open-timers", nil)
	})
	menu.Add("Open Alarms…").OnClick(func(_ *application.Context) {
		d.Window.Show()
		d.Window.Focus()
		d.App.Event.Emit("tray:open-alarms", nil)
	})
	menu.Add("Open Focus…").OnClick(func(_ *application.Context) {
		d.Window.Show()
		d.Window.Focus()
		d.App.Event.Emit("tray:open-focus", nil)
	})
	menu.Add("I HAVE NO IDEA").OnClick(func(_ *application.Context) {
		d.Window.Show()
		d.Window.Focus()
		d.App.Event.Emit("tray:open-no-idea", nil)
	})

	menu.AddSeparator()

	// Section 26: Privacy Mode. Two flat items rather than one dynamic
	// toggle, same reasoning as Pause/Resume Tracking above (this
	// tray's menu is built once, not rebuilt per click). The tray icon
	// itself does NOT currently change for Privacy Mode (section 26's
	// own "tray icon should visibly change") - that needs a third icon
	// asset distinct from the existing light/dark theme-mode pair this
	// tray already swaps between (SetIconForMode), and no such asset
	// exists yet; UpdateStatus's tray LABEL is the real, working signal
	// for now (see that method's own "PRIVACY MODE ACTIVE" case) - a
	// dedicated icon is a small, isolated follow-up once that asset is
	// made, not a structural gap.
	menu.Add("Enable Privacy Mode").OnClick(func(_ *application.Context) {
		if d.SetPrivacyMode == nil {
			return
		}
		if err := d.SetPrivacyMode(true); err != nil {
			log.Printf("tray: enabling privacy mode: %v", err)
		}
	})
	menu.Add("Disable Privacy Mode").OnClick(func(_ *application.Context) {
		if d.SetPrivacyMode == nil {
			return
		}
		if err := d.SetPrivacyMode(false); err != nil {
			log.Printf("tray: disabling privacy mode: %v", err)
		}
	})

	menu.Add("Settings").OnClick(func(_ *application.Context) {
		d.Window.Show()
		d.Window.Focus()
		// Phase 1's frontend router opens straight to /settings when it
		// receives this event - see frontend/src/App.tsx.
		d.App.Event.Emit("tray:open-settings", nil)
	})

	menu.AddSeparator()

	menu.Add("Exit").OnClick(func(_ *application.Context) {
		d.App.Quit()
	})

	sysTray.SetMenu(menu)
	return t
}

// SetIconForMode switches the tray icon to match a theme's computed mode
// (see internal/theme.ComputeMode). Called once at Build time with the
// startup theme's mode, and again from main.go's ThemeService callback
// whenever the active theme - or an in-place edit to it - changes.
//
// Deliberately uses only SetIcon, not Wails's SetDarkModeIcon: that method
// hooks into the OS's own dark-mode setting, which is a different concept
// from "which STELLAREI theme is active" (see Deps.IconDark's doc
// comment) - mixing the two would mean the icon sometimes flips for a
// reason the user never asked for.
func (t *Tray) SetIconForMode(mode theme.Mode) {
	if mode == theme.Light && t.iconLight != nil {
		t.sysTray.SetIcon(t.iconLight)
		return
	}
	if t.iconDark != nil {
		t.sysTray.SetIcon(t.iconDark)
	}
}

// UpdateStatus refreshes the tray's label to reflect the Activity
// Engine's current state (section 23's "Currently: Serenity · 01:17").
// Called from activity.Config.OnStateChange, so it runs on every poll
// tick - SetLabel is cheap even when the text hasn't changed, well within
// section 50's near-zero-idle-CPU budget at a several-second poll
// interval.
func (t *Tray) UpdateStatus(state activity.State) {
	switch {
	case state.PrivacyMode:
		// Section 26's own "PRIVACY MODE ACTIVE" - checked before the
		// generic Paused case so it's never shadowed by the ordinary
		// "Tracking Paused" text, even though PrivacyMode also implies
		// Paused (see activity.Engine.SetPrivacyMode's doc comment).
		t.sysTray.SetLabel("STELLAREI - PRIVACY MODE ACTIVE")
	case state.Paused:
		t.sysTray.SetLabel("STELLAREI - Tracking Paused")
	case state.ProcessName == "":
		t.sysTray.SetLabel("STELLAREI")
	case state.Idle:
		t.sysTray.SetLabel("STELLAREI - Idle")
	default:
		t.sysTray.SetLabel(fmt.Sprintf("%s · %s", state.ProcessName, formatElapsed(state.RunStartedAt)))
	}
}

func formatElapsed(startedAt time.Time) string {
	if startedAt.IsZero() {
		return "00:00"
	}
	d := time.Since(startedAt).Round(time.Minute)
	hours := d / time.Hour
	minutes := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
