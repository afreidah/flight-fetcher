// -------------------------------------------------------------------------------
// API Client - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the shared HTTP client: defaults, request construction, backoff on
// 429 responses, body size limiting, JSON decoding, and DoRaw bypass.
// -------------------------------------------------------------------------------

package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNew_Defaults verifies that sensible defaults are applied.
func TestNew_Defaults(t *testing.T) {
	c := New(Options{BaseURL: "https://example.com"})
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want https://example.com", c.baseURL)
	}
	if c.maxBodyBytes != defaultMaxBody {
		t.Errorf("maxBodyBytes = %d, want %d", c.maxBodyBytes, defaultMaxBody)
	}
	if c.httpClient.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", c.httpClient.Timeout, defaultTimeout)
	}
	if c.backoff != initialBackoff {
		t.Errorf("backoff = %v, want %v", c.backoff, initialBackoff)
	}
}

// TestNew_CustomOptions verifies that custom options override defaults.
func TestNew_CustomOptions(t *testing.T) {
	c := New(Options{
		BaseURL:      "https://custom.api",
		Timeout:      5 * time.Second,
		MaxBodyBytes: 50 * 1024 * 1024,
	})
	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", c.httpClient.Timeout)
	}
	if c.maxBodyBytes != 50*1024*1024 {
		t.Errorf("maxBodyBytes = %d, want 50MB", c.maxBodyBytes)
	}
}

// TestNewRequest_PrependsBaseURL verifies that the base URL is prepended to the path.
func TestNewRequest_PrependsBaseURL(t *testing.T) {
	c := New(Options{BaseURL: "https://api.example.com"})
	req, err := c.NewRequest(context.Background(), http.MethodGet, "/v1/things", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	want := "https://api.example.com/v1/things"
	if req.URL.String() != want {
		t.Errorf("URL = %q, want %q", req.URL.String(), want)
	}
}

// TestNewRequest_BlockedByBackoff verifies that requests are rejected during backoff.
func TestNewRequest_BlockedByBackoff(t *testing.T) {
	c := New(Options{BaseURL: "https://example.com"})
	c.backoffUtil = time.Now().Add(time.Hour)

	_, err := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("NewRequest() expected backoff error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %q, want it to contain 'rate limited'", err.Error())
	}
}

// TestDo_Success verifies that a successful request resets backoff and returns the response.
func TestDo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	// Simulate prior elevated backoff
	c.backoff = 5 * time.Minute

	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	resp.Body.Close()

	if c.backoff != initialBackoff {
		t.Errorf("backoff = %v, want %v after successful request", c.backoff, initialBackoff)
	}
}

// TestDo_RateLimit verifies that a 429 response triggers backoff.
func TestDo_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})

	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Fatal("Do() expected error for 429, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %q, want it to contain 'rate limited'", err.Error())
	}

	// Backoff should now block the next NewRequest
	_, err = c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("NewRequest() expected backoff error after 429, got nil")
	}
}

// TestDo_ServerError verifies that a 5xx response triggers backoff.
func TestDo_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})

	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Fatal("Do() expected error for 502, got nil")
	}
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("error = %q, want it to contain 'server error'", err.Error())
	}

	// Backoff should now block the next NewRequest
	_, err = c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("NewRequest() expected backoff error after 5xx, got nil")
	}
}

// TestDo_TransportError verifies that a connection failure triggers backoff.
func TestDo_TransportError(t *testing.T) {
	// Point at a port nothing is listening on
	c := New(Options{BaseURL: "http://127.0.0.1:1"})

	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Fatal("Do() expected error for connection refused, got nil")
	}

	// Backoff should now block the next NewRequest
	_, err = c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("NewRequest() expected backoff error after transport failure, got nil")
	}
}

// TestDo_ClientError_NoBackoff verifies that 4xx responses (except 429) do not trigger backoff.
func TestDo_ClientError_NoBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})

	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil for 404", err)
	}
	resp.Body.Close()

	// Should NOT be in backoff — 4xx is a caller problem, not the server's
	req, err = c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("NewRequest() should not be blocked after 404, got: %v", err)
	}
	resp, _ = c.Do(req)
	resp.Body.Close()
}

// TestDo_RateLimitWithRetryAfter verifies that a 429 with Retry-After header
// uses the server-specified duration instead of exponential backoff.
func TestDo_RateLimitWithRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})

	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Fatal("Do() expected error for 429, got nil")
	}

	// Backoff should be set to 120s from Retry-After, not the default 30s
	if c.backoff != 120*time.Second {
		t.Errorf("backoff = %v, want 2m0s (from Retry-After header)", c.backoff)
	}
}

// TestDo_BackoffEscalates verifies that repeated 429s double the backoff up to max.
func TestDo_BackoffEscalates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})

	for range 5 {
		c.backoffUtil = time.Time{} // clear so we can make the request
		req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
		_, _ = c.Do(req)
	}

	// After 5 doublings from 30s: 30->60->120->240->480->600 (capped at maxBackoff)
	if c.backoff != maxBackoff {
		t.Errorf("backoff = %v, want %v (should cap at max)", c.backoff, maxBackoff)
	}
}

// TestDoRaw_SkipsBackoff verifies that DoRaw does not check or modify backoff state.
func TestDoRaw_SkipsBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	c.backoff = 5 * time.Minute

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	resp, err := c.DoRaw(req)
	if err != nil {
		t.Fatalf("DoRaw() error = %v", err)
	}
	resp.Body.Close()

	// DoRaw should not have reset backoff
	if c.backoff != 5*time.Minute {
		t.Errorf("backoff = %v, want 5m (DoRaw should not modify backoff)", c.backoff)
	}
}

// TestDecodeJSON_Success verifies that JSON is decoded correctly.
func TestDecodeJSON_Success(t *testing.T) {
	c := New(Options{})
	body := strings.NewReader(`{"name":"test","value":42}`)

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := c.DecodeJSON(body, &result); err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if result.Name != "test" {
		t.Errorf("Name = %q, want %q", result.Name, "test")
	}
	if result.Value != 42 {
		t.Errorf("Value = %d, want 42", result.Value)
	}
}

// TestDecodeJSON_InvalidJSON verifies that malformed JSON returns an error.
func TestDecodeJSON_InvalidJSON(t *testing.T) {
	c := New(Options{})
	body := strings.NewReader(`not json`)

	var result struct{}
	if err := c.DecodeJSON(body, &result); err == nil {
		t.Error("DecodeJSON() expected error for invalid JSON, got nil")
	}
}

// TestDecodeJSON_BodySizeLimit verifies that oversized responses are truncated.
func TestDecodeJSON_BodySizeLimit(t *testing.T) {
	c := New(Options{MaxBodyBytes: 10})
	// Valid JSON but larger than 10 bytes
	body := strings.NewReader(`{"name":"this is a very long string that exceeds the limit"}`)

	var result struct {
		Name string `json:"name"`
	}
	if err := c.DecodeJSON(body, &result); err == nil {
		t.Error("DecodeJSON() expected error for oversized body, got nil")
	}
}

// TestParseRetryAfter verifies Retry-After header parsing.
func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"60", 60 * time.Second},
		{"0", 0},
		{"-1", 0},
		{"", 0},
		{"not-a-number", 0},
	}
	for _, tt := range tests {
		got := parseRetryAfter(tt.input)
		if got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
