// -------------------------------------------------------------------------------
// OpenSky - API Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the OpenSky Network REST API. Queries aircraft state vectors
// within a geographic bounding box using API client credentials. Malformed
// state vectors are logged and skipped rather than failing the entire response.
// -------------------------------------------------------------------------------

package opensky

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/afreidah/flight-fetcher/internal/geo"
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
	}
}

// GetStates queries OpenSky for aircraft state vectors within the given
// bounding box. Malformed individual state vectors are logged and skipped.
func (c *Client) GetStates(ctx context.Context, bbox geo.BBox) (*StatesResponse, error) {
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var raw struct {
		Time   int64              `json:"time"`
		States []json.RawMessage  `json:"states"`
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
