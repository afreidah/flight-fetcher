// -------------------------------------------------------------------------------
// Discord - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests Discord webhook notification delivery: successful sends, server
// errors, and payload formatting.
// -------------------------------------------------------------------------------

package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/notify"
)

// TestSend_Success verifies that a message is posted as a Discord embed.
func TestSend_Success(t *testing.T) {
	var got webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
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

	if len(got.Embeds) != 1 {
		t.Fatalf("embeds count = %d, want 1", len(got.Embeds))
	}
	e := got.Embeds[0]
	if e.Title != "Emergency Squawk" {
		t.Errorf("title = %q, want %q", e.Title, "Emergency Squawk")
	}
	if e.Color != embedColor {
		t.Errorf("color = %d, want %d", e.Color, embedColor)
	}
	if len(e.Fields) != 2 {
		t.Fatalf("fields count = %d, want 2", len(e.Fields))
	}
	if e.Fields[0].Name != "ICAO24" || e.Fields[0].Value != "abc123" {
		t.Errorf("field[0] = %+v, want ICAO24/abc123", e.Fields[0])
	}
}

// TestSend_ServerError verifies that a non-2xx response returns an error.
func TestSend_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.Send(context.Background(), notify.Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error for 500 response, got nil")
	}
}

// TestSend_NoFields verifies that a message without fields omits the fields array.
func TestSend_NoFields(t *testing.T) {
	var got webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.Send(context.Background(), notify.Message{Title: "simple", Body: "no fields"})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(got.Embeds[0].Fields) != 0 {
		t.Errorf("fields count = %d, want 0", len(got.Embeds[0].Fields))
	}
}

// TestSend_TransportError verifies that a connection failure returns an error.
func TestSend_TransportError(t *testing.T) {
	c := New("http://127.0.0.1:1") // nothing listening
	err := c.Send(context.Background(), notify.Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error for unreachable server, got nil")
	}
}

// TestSend_InvalidURL verifies that a bad webhook URL returns an error.
func TestSend_InvalidURL(t *testing.T) {
	c := New("://bad-url")
	err := c.Send(context.Background(), notify.Message{Title: "test"})
	if err == nil {
		t.Fatal("Send() expected error for invalid URL, got nil")
	}
}
