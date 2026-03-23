// -------------------------------------------------------------------------------
// HexDB - API Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the HexDB.io aircraft metadata API. Performs lookups by
// ICAO24 hex code to retrieve aircraft registration, type, manufacturer, and
// operator information. No authentication required.
// -------------------------------------------------------------------------------

package hexdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the HexDB.io API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates a HexDB.io API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://hexdb.io/api/v1",
	}
}

// Lookup fetches aircraft metadata by ICAO24 hex code. Returns nil if the
// aircraft is not found in HexDB.
func (c *Client) Lookup(ctx context.Context, icao24 string) (*AircraftInfo, error) {
	url := fmt.Sprintf("%s/aircraft/%s", c.baseURL, icao24)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var info AircraftInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	info.ICAO24 = icao24
	return &info, nil
}
