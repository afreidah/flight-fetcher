// -------------------------------------------------------------------------------
// AirLabs - API Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the AirLabs flight data API. Looks up flight route
// information by ICAO flight code (callsign) to retrieve departure and
// arrival airports. Requires an API key from https://airlabs.co.
// -------------------------------------------------------------------------------

package airlabs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/afreidah/flight-fetcher/internal/route"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the AirLabs API.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// apiResponse wraps the AirLabs JSON response envelope.
type apiResponse struct {
	Response route.Info `json:"response"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates an AirLabs API client with the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		baseURL:    "https://airlabs.co/api/v9",
	}
}

// LookupRoute fetches route information for a flight by ICAO flight code
// (callsign). Returns nil if the flight is not found.
func (c *Client) LookupRoute(ctx context.Context, callsign string) (*route.Info, error) {
	callsign = strings.TrimSpace(callsign)
	if callsign == "" {
		return nil, nil
	}

	params := url.Values{
		"flight_icao": {callsign},
		"api_key":     {c.apiKey},
	}
	reqURL := fmt.Sprintf("%s/flight?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Empty response when flight not found
	if result.Response.FlightICAO == "" {
		return nil, nil
	}

	return &result.Response, nil
}
