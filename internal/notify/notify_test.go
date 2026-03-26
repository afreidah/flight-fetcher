// -------------------------------------------------------------------------------
// Notify - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the notification Manager: fan-out to multiple backends, no-op when
// empty, error collection, and Register.
// -------------------------------------------------------------------------------

package notify

import (
	"context"
	"errors"
	"testing"
)

// stubNotifier records calls and optionally returns an error.
type stubNotifier struct {
	messages []Message
	err      error
}

func (s *stubNotifier) Send(_ context.Context, msg Message) error {
	s.messages = append(s.messages, msg)
	return s.err
}

// TestManager_NoNotifiers verifies that an empty manager is a no-op.
func TestManager_NoNotifiers(t *testing.T) {
	mgr := NewManager()
	if err := mgr.Send(context.Background(), Message{Title: "test"}); err != nil {
		t.Errorf("Send() error = %v, want nil for empty manager", err)
	}
}

// TestManager_FanOut verifies that all registered notifiers receive the message.
func TestManager_FanOut(t *testing.T) {
	a := &stubNotifier{}
	b := &stubNotifier{}
	mgr := NewManager(a, b)

	msg := Message{Title: "alert", Body: "test"}
	if err := mgr.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(a.messages) != 1 {
		t.Errorf("notifier a got %d messages, want 1", len(a.messages))
	}
	if len(b.messages) != 1 {
		t.Errorf("notifier b got %d messages, want 1", len(b.messages))
	}
	if a.messages[0].Title != "alert" {
		t.Errorf("title = %q, want %q", a.messages[0].Title, "alert")
	}
}

// TestManager_Register verifies that Register adds backends after creation.
func TestManager_Register(t *testing.T) {
	mgr := NewManager()
	n := &stubNotifier{}
	mgr.Register(n)

	if err := mgr.Send(context.Background(), Message{Title: "test"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(n.messages) != 1 {
		t.Errorf("got %d messages, want 1", len(n.messages))
	}
}

// TestManager_PartialError verifies that one failure doesn't block others.
func TestManager_PartialError(t *testing.T) {
	good := &stubNotifier{}
	bad := &stubNotifier{err: errors.New("fail")}
	mgr := NewManager(bad, good)

	err := mgr.Send(context.Background(), Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error, got nil")
	}
	// Both should have been called despite the error
	if len(bad.messages) != 1 {
		t.Errorf("bad notifier got %d messages, want 1", len(bad.messages))
	}
	if len(good.messages) != 1 {
		t.Errorf("good notifier got %d messages, want 1", len(good.messages))
	}
}
