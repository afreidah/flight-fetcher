// -------------------------------------------------------------------------------
// OpenSky - API Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the OpenSky Network REST API. Queries aircraft state vectors
// within a geographic bounding box using API client credentials. Malformed
// state vectors are logged and skipped. Applies exponential backoff on HTTP 429
// rate limit responses to avoid hammering the API.
// -------------------------------------------------------------------------------

package opensky

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/afreidah/flight-fetcher/internal/geo"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

// backoff parameters for rate limit handling.
const (
	initialBackoff = 30 * time.Second
	maxBackoff     = 10 * time.Minute
	backoffFactor  = 2
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the OpenSky Network API.
type Client struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	baseURL      string

	mu          sync.Mutex
	backoffUtil time.Time
	backoff     time.Duration
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates an OpenSky API client with the given API client credentials.
func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		httpClient:   &http.Client{},
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      "https://opensky-network.org/api",
		backoff:      initialBackoff,
	}
}

// GetStates queries OpenSky for aircraft state vectors within the given
// bounding box. Malformed individual state vectors are logged and skipped.
// Returns a rate limit error without making a request if the client is in
// a backoff period.
func (c *Client) GetStates(ctx context.Context, bbox geo.BBox) (*StatesResponse, error) {
	if err := c.checkBackoff(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/states/all?lamin=%f&lomin=%f&lamax=%f&lomax=%f",
		c.baseURL, bbox.MinLat, bbox.MinLon, bbox.MaxLat, bbox.MaxLon)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if c.clientID != "" {
		req.SetBasicAuth(c.clientID, c.clientSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.applyBackoff(ctx)
		return nil, fmt.Errorf("rate limited (429), backing off for %s", c.currentBackoff())
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	c.resetBackoff()

	var raw struct {
		Time   int64             `json:"time"`
		States []json.RawMessage `json:"states"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	result := &StatesResponse{Time: raw.Time, States: make([]StateVector, 0, len(raw.States))}
	for i, s := range raw.States {
		var sv StateVector
		if err := json.Unmarshal(s, &sv); err != nil {
			slog.WarnContext(ctx, "skipping malformed state vector",
				slog.Int("index", i),
				slog.String("error", err.Error()))
			continue
		}
		result.States = append(result.States, sv)
	}
	return result, nil
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
