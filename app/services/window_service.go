package services

import "github.com/wailsapp/wails/v3/pkg/application"

// WindowService exposes the main window's lifecycle to the frontend
// specifically the three choices in the "Ask every time" close prompt
// (section 73): Close STELLAREI, Minimize to Tray, Cancel. Cancel needs no
// backend call (the frontend just dismisses its own dialog), so only two
// methods are exposed here.
//
// Construction order problem this works around: application.Options must
// list every Service before application.New returns an *application.App,
// but WindowService's methods need that *application.App (and the main
// window, created afterwards) to do anything. Rather than restructure
// Wails's API or reach for a global, WindowService is constructed empty
// and wired up via Bind once both dependencies exist - see main.go. Every
// method below is a no-op until Bind has run, which in practice is before
// app.Run() starts serving the frontend, so the frontend can never
// observe the unbound state.
type WindowService struct {
	app    *application.App
	window *application.WebviewWindow
}

// NewWindowService constructs an unbound WindowService, suitable for
// passing straight into application.Options.Services.
func NewWindowService() *WindowService {
	return &WindowService{}
}

// Bind supplies the App and main WebviewWindow once they exist. Must be
// called exactly once, after application.New and app.Window.NewWithOptions,
// before app.Run().
func (w *WindowService) Bind(app *application.App, window *application.WebviewWindow) {
	w.app = app
	w.window = window
}

// MinimizeToTray hides the main window; the process and every background
// service keep running (section 73, "Minimize to Tray").
func (w *WindowService) MinimizeToTray() {
	if w.window != nil {
		w.window.Hide()
	}
}

// QuitApplication fully exits STELLAREI (section 73, "Close STELLAREI").
func (w *WindowService) QuitApplication() {
	if w.app != nil {
		w.app.Quit()
	}
}

// Show un-hides and focuses the main window. Exposed for the tray's "Show
// STELLAREI" item to have a frontend-callable equivalent (useful from
// devtools during development, and by any future in-app "reopen" button).
func (w *WindowService) Show() {
	if w.window != nil {
		w.window.Show()
		w.window.Focus()
	}
}
