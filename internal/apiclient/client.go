// -------------------------------------------------------------------------------
// API Client - Shared HTTP Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Provides common HTTP client functionality for external API integrations:
// request construction with base URL, exponential backoff on 429 responses,
// response body size limiting, and JSON decoding. Embedded by each API-specific
// client to avoid duplicating these concerns.
// -------------------------------------------------------------------------------

package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	defaultTimeout  = 10 * time.Second
	defaultMaxBody  = 1 * 1024 * 1024 // 1MB
	initialBackoff  = 30 * time.Second
	maxBackoff      = 10 * time.Minute
	backoffFactor   = 2
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Options configures a new Client.
type Options struct {
	BaseURL      string
	Timeout      time.Duration
	MaxBodyBytes int64
}

// Client provides shared HTTP functionality for external API integrations.
// Handles exponential backoff on rate limits, response body size limiting,
// and JSON decoding. Intended to be embedded by API-specific clients.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	maxBodyBytes int64

	mu          sync.Mutex
	backoff     time.Duration
	backoffUtil time.Time
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Client with the given options. Applies sensible defaults
// for timeout (10s) and max body size (1MB) when not specified.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultMaxBody
	}
	return &Client{
		httpClient:   &http.Client{Timeout: opts.Timeout},
		baseURL:      opts.BaseURL,
		maxBodyBytes: opts.MaxBodyBytes,
		backoff:      initialBackoff,
	}
}

// NewRequest creates an HTTP request with the base URL prepended to path.
// Returns an error if the client is in a backoff period.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if err := c.checkBackoff(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	return req, nil
}

// Do executes a request, applying exponential backoff on 429 responses.
// On rate limit, the response body is closed and an error is returned.
// The caller must close resp.Body on success.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		c.applyBackoff(req.Context())
		return nil, fmt.Errorf("rate limited (429), backing off for %s", c.currentBackoff())
	}
	c.resetBackoff()
	return resp, nil
}

// DoRaw executes a request without backoff checking or handling. Used for
// auxiliary requests like OAuth2 token endpoints that should not participate
// in the main API's backoff state.
func (c *Client) DoRaw(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// DecodeJSON reads from r with the configured body size limit and decodes
// JSON into v.
func (c *Client) DecodeJSON(r io.Reader, v any) error {
	if err := json.NewDecoder(io.LimitReader(r, c.maxBodyBytes)).Decode(v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// checkBackoff returns an error if the client is currently in a backoff
// period, skipping the request entirely.
func (c *Client) checkBackoff(ctx context.Context) error {
	c.mu.Lock()
	until := c.backoffUtil
	c.mu.Unlock()

	if time.Now().Before(until) {
		remaining := time.Until(until).Truncate(time.Second)
		slog.InfoContext(ctx, "skipping request during backoff",
			slog.String("remaining", remaining.String()))
		return fmt.Errorf("rate limited, backoff expires in %s", remaining)
	}
	return nil
}

// applyBackoff doubles the backoff duration (up to maxBackoff) and sets the
// next allowed request time.
func (c *Client) applyBackoff(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.backoffUtil = time.Now().Add(c.backoff)
	slog.WarnContext(ctx, "rate limited, applying backoff",
		slog.String("duration", c.backoff.String()))

	c.backoff *= backoffFactor
	if c.backoff > maxBackoff {
		c.backoff = maxBackoff
	}
}

// resetBackoff clears the backoff state after a successful request.
func (c *Client) resetBackoff() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.backoff = initialBackoff
	c.backoffUtil = time.Time{}
}

// currentBackoff returns the current backoff duration for logging.
func (c *Client) currentBackoff() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.backoff
}
