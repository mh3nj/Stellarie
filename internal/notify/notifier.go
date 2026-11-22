package notify

import (
	"context"
	"log"
)

// Notifier combines persisting a notification to history (Repository)
// with actually showing it natively (Dispatcher) - the single entry
// point internal/scheduler calls, so it never has to remember to do both
// separately or risk doing them in an inconsistent order.
type Notifier struct {
	repo       *Repository
	dispatcher Dispatcher
}

func NewNotifier(repo *Repository, dispatcher Dispatcher) *Notifier {
	if dispatcher == nil {
		dispatcher = NoopDispatcher{}
	}
	return &Notifier{repo: repo, dispatcher: dispatcher}
}

// Notify logs the notification to history unconditionally, then attempts
// native delivery. History is written first and independently: per
// section 56, a native-delivery failure (no notification daemon running,
// macOS authorization denied, etc.) must not mean the event itself goes
// unrecorded - the in-app notification history should still show "your
// Gaming budget hit its limit" even if the OS banner never appeared.
func (n *Notifier) Notify(ctx context.Context, source Source, sourceID int64, title, body, sound string) error {
	if _, err := n.repo.Insert(ctx, source, sourceID, title, body); err != nil {
		log.Printf("notify: logging notification history: %v", err)
		// Deliberately not returned: history logging is a nice-to-have
		// audit trail, not something that should block/skip actually
		// telling the person via the native banner below.
	}
	return n.dispatcher.Send(title, body, sound)
}
