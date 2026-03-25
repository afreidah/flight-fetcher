// -------------------------------------------------------------------------------
// OpenSky - API Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the OpenSky Network REST API. Authenticates via OAuth2 client
// credentials flow, caches the access token until expiry. Rate limiting and
// exponential backoff are handled by the embedded apiclient.Client.
// -------------------------------------------------------------------------------

package opensky

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient"
	"github.com/afreidah/flight-fetcher/internal/geo"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

// defaultTokenURL is the OpenSky OAuth2 token endpoint.
const defaultTokenURL = "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the OpenSky Network API.
type Client struct {
	*apiclient.Client
	clientID     string
	clientSecret string
	tokenURL     string

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

// tokenResponse represents the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates an OpenSky API client with the given OAuth2 client credentials.
func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		Client: apiclient.New(apiclient.Options{
			Name:         "opensky",
			BaseURL:      "https://opensky-network.org/api",
			Timeout:      15 * time.Second,
			MaxBodyBytes: 50 * 1024 * 1024, // 50MB for global queries
		}),
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     defaultTokenURL,
	}
}

// GetStates queries OpenSky for aircraft state vectors within the given
// bounding box. Malformed individual state vectors are logged and skipped.
// Returns a rate limit error without making a request if the client is in
// a backoff period.
func (c *Client) GetStates(ctx context.Context, bbox geo.BBox) (*StatesResponse, error) {
	path := fmt.Sprintf("/states/all?lamin=%f&lomin=%f&lamax=%f&lomax=%f",
		bbox.MinLat, bbox.MinLon, bbox.MaxLat, bbox.MaxLon)

	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	if c.clientID != "" {
		token, err := c.getToken(ctx)
		if err != nil {
			slog.WarnContext(ctx, "oauth2 token fetch failed, trying without auth",
				slog.String("error", err.Error()))
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var raw struct {
		Time   int64             `json:"time"`
		States []json.RawMessage `json:"states"`
	}
	if err := c.DecodeJSON(resp.Body, &raw); err != nil {
		return nil, err
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

// getToken returns a cached OAuth2 access token, refreshing it if expired.
// Uses a dedicated mutex so only one goroutine refreshes at a time while
// the backoff mutex remains unblocked. Uses DoRaw to avoid participating
// in the main API's backoff state.
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Re-check under lock - another goroutine may have refreshed while we waited
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.DoRaw(req)
	if err != nil {
		return "", fmt.Errorf("executing token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := c.DecodeJSON(resp.Body, &tr); err != nil {
		return "", err
	}

	c.token = tr.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn)*time.Second - 30*time.Second)

	slog.InfoContext(ctx, "oauth2 token acquired",
		slog.Int("expires_in_sec", tr.ExpiresIn))

	return tr.AccessToken, nil
}
