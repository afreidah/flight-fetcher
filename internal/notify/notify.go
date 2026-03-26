// -------------------------------------------------------------------------------
// Notify - Notification Interface and Manager
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the generic notification interface, message types, and a Manager
// that fans out messages to all registered backends (Discord, Telegram, etc.).
// The Manager itself satisfies Notifier, so consumers take a single Notifier.
// -------------------------------------------------------------------------------

package notify

import (
	"context"
	"errors"
	"log/slog"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Notifier sends a notification message through an external service.
type Notifier interface {
	Send(ctx context.Context, msg Message) error
}

// Message is a notification payload with optional structured fields for
// rich formatting (e.g., Discord embeds, Slack blocks).
type Message struct {
	Title  string
	Body   string
	Fields []Field
}

// Field is an ordered key-value pair for structured notification content.
type Field struct {
	Name  string
	Value string
}

// Manager fans out notifications to all registered backends. Implements
// Notifier so consumers don't need to know about multiple backends.
// A Manager with no registered notifiers is a no-op.
type Manager struct {
	notifiers []Notifier
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewManager creates a Manager with the given notifiers.
func NewManager(notifiers ...Notifier) *Manager {
	return &Manager{notifiers: notifiers}
}

// Register adds a notifier backend to the manager.
func (m *Manager) Register(n Notifier) {
	m.notifiers = append(m.notifiers, n)
}

// Send fans out the message to all registered notifiers. Errors are logged
// and collected but do not stop delivery to remaining backends.
func (m *Manager) Send(ctx context.Context, msg Message) error {
	if len(m.notifiers) == 0 {
		return nil
	}
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Send(ctx, msg); err != nil {
			slog.WarnContext(ctx, "notification send failed",
				slog.String("error", err.Error()))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
