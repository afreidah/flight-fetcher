// -------------------------------------------------------------------------------
// FlightAware - AeroAPI Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for the FlightAware AeroAPI. Looks up flight route information
// by callsign (ident) to retrieve departure and arrival airports. Used as a
// fallback when the primary route lookup (AirLabs) is unavailable.
// -------------------------------------------------------------------------------

package flightaware

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/afreidah/flight-fetcher/internal/apiclient"
	"github.com/afreidah/flight-fetcher/internal/route"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the FlightAware AeroAPI.
type Client struct {
	*apiclient.Client
	apiKey string
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates a FlightAware AeroAPI client with the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		Client: apiclient.New(apiclient.Options{
			Name:    "flightaware",
			BaseURL: "https://aeroapi.flightaware.com/aeroapi",
		}),
		apiKey: apiKey,
	}
}

// LookupRoute fetches route information for a flight by callsign (ident).
// Returns a route.Info for compatibility with the existing enricher
// interface. Returns nil if the flight is not found.
func (c *Client) LookupRoute(ctx context.Context, callsign string) (*route.Info, error) {
	callsign = strings.TrimSpace(callsign)
	if callsign == "" {
		return nil, nil
	}

	req, err := c.NewRequest(ctx, http.MethodGet, "/flights/"+url.PathEscape(callsign), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apikey", c.apiKey)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result flightsResponse
	if err := c.DecodeJSON(resp.Body, &result); err != nil {
		return nil, err
	}

	if len(result.Flights) == 0 {
		return nil, nil
	}

	f := result.Flights[0]
	return &route.Info{
		FlightICAO: callsign,
		DepIATA:    f.Origin.CodeIATA,
		DepICAO:    f.Origin.CodeICAO,
		DepName:    f.Origin.Name,
		ArrIATA:    f.Destination.CodeIATA,
		ArrICAO:    f.Destination.CodeICAO,
		ArrName:    f.Destination.Name,
	}, nil
}
