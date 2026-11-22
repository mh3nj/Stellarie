package platform

// ScreenCapturer takes a full-screen screenshot and saves it as a PNG at
// path. Every implementation must fail fast with ErrUnavailable rather
// than hang or crash when the underlying tool/API isn't available - the
// same contract ActiveWindowWatcher and IdleWatcher already follow, so
// internal/privacy's sweeper can treat "couldn't capture this time" as
// routine, not exceptional (section 56).
type ScreenCapturer interface {
	CaptureScreen(path string) error
}

// NewScreenCapturer is implemented once per OS build, exactly like
// NewActiveWindowWatcher/NewIdleWatcher/NewStartupManager.
func NewScreenCapturer() ScreenCapturer {
	return newScreenCapturer()
}
