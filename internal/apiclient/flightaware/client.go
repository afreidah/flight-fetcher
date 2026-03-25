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
// Returns nil if the flight is not found.
func (c *Client) LookupRoute(ctx context.Context, callsign string) (*route.Info, error) {
	callsign = strings.TrimSpace(callsign)
	if callsign == "" {
		return nil, nil
	}

	result, err := apiclient.Lookup[flightsResponse](c.Client, ctx,
		"/flights/"+url.PathEscape(callsign),
		func(req *http.Request) { req.Header.Set("x-apikey", c.apiKey) })
	if err != nil || result == nil {
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
