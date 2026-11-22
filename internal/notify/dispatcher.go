package notify

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// notificationSeq guarantees a unique suffix even for two notifications
// delivered within the same nanosecond. Atomic so concurrent senders (the
// scheduler's tick goroutine plus any future caller) can never collide.
var notificationSeq atomic.Uint64

// nextNotificationID returns the ID every notification sent through this
// package carries. Wails' NotificationService.SendNotification rejects an
// options value with an empty ID before it ever reaches the platform, so
// leaving it unset meant NOTHING was ever shown: no alarm banner, no timer
// completion, no focus warning. The ID is opaque for an immediate send; it
// is a stable, unique handle, which is exactly what the field is for.
func nextNotificationID() string {
	return fmt.Sprintf("stellarei-%d-%d", time.Now().UnixNano(), notificationSeq.Add(1))
}

// buildOptions assembles the Wails options for one notification. Split out
// of Send so the ID/title contract is unit-testable without a running Wails
// application (see dispatcher_test.go).
func buildOptions(title, body, sound string) notifications.NotificationOptions {
	opts := notifications.NotificationOptions{
		ID:    nextNotificationID(),
		Title: title,
		Body:  body,
	}
	if sound == "silent" {
		opts.Sound = &notifications.NotificationSound{Silent: true}
	} else if sound != "" {
		opts.Sound = &notifications.NotificationSound{Name: sound}
	}
	return opts
}

// Dispatcher sends a native OS notification. Interface (not the concrete
// *notifications.NotificationService directly) so internal/scheduler's
// tests can inject a fake instead of touching real OS notification APIs
// - the same reason internal/activity.Engine takes ProjectResolver as an
// interface rather than depending on internal/projects.Resolver's
// concrete type.
type Dispatcher interface {
	Send(title, body, sound string) error
}

// WailsDispatcher adapts the real Wails v3 notification service
// (github.com/wailsapp/wails/v3/pkg/services/notifications - confirmed
// against the live package docs: NotificationOptions{Title, Body, Sound
// *NotificationSound}, (*NotificationService).SendNotification(options)
// error) to the Dispatcher interface. Constructed once in main.go around
// the same *notifications.NotificationService instance that's registered
// as a Wails application.Service, so notification delivery and (if this
// app ever calls RequestNotificationAuthorization from the frontend too)
// authorization state stay on one shared instance rather than two.
type WailsDispatcher struct {
	svc *notifications.NotificationService
}

// NewWailsDispatcher wraps an already-constructed notification service.
// Doesn't call notifications.New() itself - main.go owns that, since the
// same instance also needs to be registered as an application.Service.
func NewWailsDispatcher(svc *notifications.NotificationService) *WailsDispatcher {
	return &WailsDispatcher{svc: svc}
}

// Send shows a native notification. sound == "" uses the platform
// default sound (nil Sound field, per the package's own documented
// nil-means-default contract) rather than silence - STELLAREI's alarms
// should be heard, not just seen, unless the person explicitly picked a
// silent sound for a specific alarm (section 22 "configurable sound
// behavior").
func (d *WailsDispatcher) Send(title, body, sound string) error {
	opts := buildOptions(title, body, sound)
	if err := d.svc.SendNotification(opts); err != nil {
		return fmt.Errorf("notify: sending native notification: %w", err)
	}
	return nil
}

// NoopDispatcher does nothing but succeed - used where a Dispatcher is
// required but native delivery genuinely isn't available (e.g. this
// sandbox has no notification daemon), rather than leaving the field nil
// and forcing every call site to nil-check. Distinct from a test fake:
// this one ships in the real binary as the fallback if constructing the
// real Wails dispatcher ever fails to initialize (section 56 - a
// notification subsystem hiccup must not stop the rest of the app).
type NoopDispatcher struct{}

func (NoopDispatcher) Send(string, string, string) error { return nil }
