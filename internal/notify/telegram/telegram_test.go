// -------------------------------------------------------------------------------
// Telegram - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests Telegram Bot API notification delivery: successful sends, API errors,
// transport errors, and message formatting.
// -------------------------------------------------------------------------------

package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/notify"
)

// TestSend_Success verifies that a message is posted to the Telegram Bot API.
func TestSend_Success(t *testing.T) {
	var got sendMessageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	c := New("test-token", "12345")
	c.baseURL = srv.URL

	err := c.Send(context.Background(), notify.Message{
		Title: "Emergency Squawk",
		Body:  "Aircraft broadcasting 7700",
		Fields: []notify.Field{
			{Name: "ICAO24", Value: "abc123"},
			{Name: "Squawk", Value: "7700"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if got.ChatID != "12345" {
		t.Errorf("chat_id = %q, want %q", got.ChatID, "12345")
	}
	if got.ParseMode != "HTML" {
		t.Errorf("parse_mode = %q, want %q", got.ParseMode, "HTML")
	}
	if got.Text == "" {
		t.Error("text is empty")
	}
}

// TestSend_APIError verifies that a Telegram API error is returned.
func TestSend_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": false, "description": "Unauthorized"}`))
	}))
	defer srv.Close()

	c := New("bad-token", "12345")
	c.baseURL = srv.URL

	err := c.Send(context.Background(), notify.Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error for API failure, got nil")
	}
}

// TestSend_TransportError verifies that a connection failure returns an error.
func TestSend_TransportError(t *testing.T) {
	c := New("token", "12345")
	c.baseURL = "http://127.0.0.1:1"

	err := c.Send(context.Background(), notify.Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error for unreachable server, got nil")
	}
}

// TestSend_InvalidURL verifies that a bad base URL returns an error.
func TestSend_InvalidURL(t *testing.T) {
	c := New("token", "12345")
	c.baseURL = "://bad-url"

	err := c.Send(context.Background(), notify.Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error for invalid URL, got nil")
	}
}

// TestFormatHTML verifies the HTML message formatting.
func TestFormatHTML(t *testing.T) {
	msg := notify.Message{
		Title: "Alert",
		Body:  "Something happened",
		Fields: []notify.Field{
			{Name: "Key1", Value: "Val1"},
			{Name: "Key2", Value: "Val2"},
		},
	}
	got := formatHTML(msg)
	want := "<b>Alert</b>\nSomething happened\n\n<b>Key1:</b> Val1\n<b>Key2:</b> Val2"
	if got != want {
		t.Errorf("formatHTML() =\n%s\nwant:\n%s", got, want)
	}
}

// TestFormatHTML_NoFields verifies formatting without fields.
func TestFormatHTML_NoFields(t *testing.T) {
	msg := notify.Message{Title: "Simple", Body: "Just a message"}
	got := formatHTML(msg)
	want := "<b>Simple</b>\nJust a message\n"
	if got != want {
		t.Errorf("formatHTML() = %q, want %q", got, want)
	}
}
