// -------------------------------------------------------------------------------
// OpenSky - API Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the OpenSky Network REST API. Queries aircraft state vectors
// within a geographic bounding box using API client credentials. Parses the raw
// heterogeneous JSON arrays into typed StateVector structs.
// -------------------------------------------------------------------------------

package opensky

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/afreidah/flight-fetcher/internal/geo"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the OpenSky Network API.
type Client struct {
	httpClient *http.Client
	clientID   string
	clientSecret string
	baseURL    string
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
// bounding box.
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
		Time   int64           `json:"time"`
		States [][]interface{} `json:"states"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	result := &StatesResponse{Time: raw.Time}
	for _, s := range raw.States {
		sv, err := parseStateVector(s)
		if err != nil {
			continue
		}
		result.States = append(result.States, sv)
	}
	return result, nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// parseStateVector converts a raw JSON array from the OpenSky API into a
// typed StateVector. Returns an error if the array is too short.
func parseStateVector(raw []interface{}) (StateVector, error) {
	if len(raw) < 17 {
		return StateVector{}, fmt.Errorf("state vector too short: %d elements", len(raw))
	}

	sv := StateVector{}
	if v, ok := raw[0].(string); ok {
		sv.ICAO24 = v
	}
	if v, ok := raw[1].(string); ok {
		sv.Callsign = v
	}
	if v, ok := raw[2].(string); ok {
		sv.OriginCountry = v
	}
	if v, ok := raw[5].(float64); ok {
		sv.Longitude = v
	}
	if v, ok := raw[6].(float64); ok {
		sv.Latitude = v
	}
	if v, ok := raw[7].(float64); ok {
		sv.BaroAltitude = v
	}
	if v, ok := raw[9].(float64); ok {
		sv.Velocity = v
	}
	if v, ok := raw[10].(float64); ok {
		sv.Heading = v
	}
	if v, ok := raw[11].(float64); ok {
		sv.VerticalRate = v
	}
	if v, ok := raw[8].(bool); ok {
		sv.OnGround = v
	}
	return sv, nil
}
