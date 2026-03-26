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

	"github.com/afreidah/flight-fetcher/internal/apiclient"
)

// TestNewClient verifies that the constructor returns a valid client.
func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
}

// testClient creates a Client pointed at the given test server.
func testClient(srv *httptest.Server) *Client {
	return &Client{
		Client:       apiclient.New(apiclient.Options{BaseURL: srv.URL}),
		imageBaseURL: srv.URL,
	}
}

// TestLookup_Success verifies that a valid HexDB response is decoded correctly
// and the image URL is resolved from the image endpoint.
func TestLookup_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/aircraft/abc123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Registration": "N12345",
				"ManufacturerName": "Boeing",
				"Type": "737-800",
				"OperatorFlagCode": "UAL",
				"ICAOTypeCode": "B738",
				"RegisteredOwners": "United Airlines"
			}`))
		case "/hex-image":
			_, _ = w.Write([]byte("https://hexdb.io/static/aircraft-images/N12345.jpg"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := testClient(srv)
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
	if info.ICAOTypeCode != "B738" {
		t.Errorf("ICAOTypeCode = %q, want %q", info.ICAOTypeCode, "B738")
	}
	if info.RegisteredOwners != "United Airlines" {
		t.Errorf("RegisteredOwners = %q, want %q", info.RegisteredOwners, "United Airlines")
	}
	if info.ImageURL != "https://hexdb.io/static/aircraft-images/N12345.jpg" {
		t.Errorf("ImageURL = %q, want resolved image URL", info.ImageURL)
	}
}

// TestLookup_NotFound verifies that a 404 response returns nil without error.
func TestLookup_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := testClient(srv)
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

	c := testClient(srv)
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

	c := testClient(srv)
	_, err := c.Lookup(context.Background(), "abc123")
	if err == nil {
		t.Error("Lookup() expected error for invalid JSON, got nil")
	}
}

// TestFetchImageURL_Success verifies that the image endpoint URL is resolved.
func TestFetchImageURL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("https://hexdb.io/static/aircraft-images/N12345.jpg"))
	}))
	defer srv.Close()

	c := testClient(srv)
	got := c.FetchImageURL(context.Background(), "abc123")
	if got != "https://hexdb.io/static/aircraft-images/N12345.jpg" {
		t.Errorf("FetchImageURL() = %q, want resolved URL", got)
	}
}

// TestFetchImageURL_NotAvailable verifies that n/a response returns empty string.
func TestFetchImageURL_NotAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("n/a"))
	}))
	defer srv.Close()

	c := testClient(srv)
	got := c.FetchImageURL(context.Background(), "abc123")
	if got != "" {
		t.Errorf("FetchImageURL() = %q, want empty for n/a", got)
	}
}

// TestFetchImageURL_ServerError verifies that a non-200 returns empty string.
func TestFetchImageURL_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := testClient(srv)
	got := c.FetchImageURL(context.Background(), "abc123")
	if got != "" {
		t.Errorf("FetchImageURL() = %q, want empty for server error", got)
	}
}

// TestFetchImageURL_TransportError verifies that a connection failure returns empty string.
func TestFetchImageURL_TransportError(t *testing.T) {
	c := testClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	c.imageBaseURL = "http://127.0.0.1:1"

	got := c.FetchImageURL(context.Background(), "abc123")
	if got != "" {
		t.Errorf("FetchImageURL() = %q, want empty for transport error", got)
	}
}
