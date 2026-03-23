// -------------------------------------------------------------------------------
// HexDB - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the HexDB.io API client response decoding, including successful
// lookups, not-found responses, and error handling.
// -------------------------------------------------------------------------------

package hexdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLookup_Success verifies that a valid HexDB response is decoded correctly.
func TestLookup_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/aircraft/abc123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Registration": "N12345",
			"ManufacturerName": "Boeing",
			"Type": "737-800",
			"OperatorFlagCode": "UAL"
		}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	info, err := c.Lookup(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if info == nil {
		t.Fatal("Lookup() returned nil")
	}
	if info.ICAO24 != "abc123" {
		t.Errorf("ICAO24 = %q, want %q", info.ICAO24, "abc123")
	}
	if info.Registration != "N12345" {
		t.Errorf("Registration = %q, want %q", info.Registration, "N12345")
	}
	if info.ManufacturerName != "Boeing" {
		t.Errorf("ManufacturerName = %q, want %q", info.ManufacturerName, "Boeing")
	}
	if info.Type != "737-800" {
		t.Errorf("Type = %q, want %q", info.Type, "737-800")
	}
	if info.OperatorFlagCode != "UAL" {
		t.Errorf("OperatorFlagCode = %q, want %q", info.OperatorFlagCode, "UAL")
	}
}

// TestLookup_NotFound verifies that a 404 response returns nil without error.
func TestLookup_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	info, err := c.Lookup(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if info != nil {
		t.Errorf("Lookup() = %v, want nil for 404", info)
	}
}

// TestLookup_ServerError verifies that a 500 response returns an error.
func TestLookup_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := c.Lookup(context.Background(), "abc123")
	if err == nil {
		t.Error("Lookup() expected error for 500 response, got nil")
	}
}

// TestLookup_InvalidJSON verifies that a malformed response returns an error.
func TestLookup_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := c.Lookup(context.Background(), "abc123")
	if err == nil {
		t.Error("Lookup() expected error for invalid JSON, got nil")
	}
}
