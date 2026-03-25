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
	"net/url"

	"github.com/afreidah/flight-fetcher/internal/apiclient"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the HexDB.io API.
type Client struct {
	*apiclient.Client
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates a HexDB.io API client.
func NewClient() *Client {
	return &Client{
		Client: apiclient.New(apiclient.Options{
			Name:    "hexdb",
			BaseURL: "https://hexdb.io/api/v1",
		}),
	}
}

// Lookup fetches aircraft metadata by ICAO24 hex code. Returns nil if the
// aircraft is not found in HexDB.
func (c *Client) Lookup(ctx context.Context, icao24 string) (*AircraftInfo, error) {
	info, err := apiclient.Lookup[AircraftInfo](c.Client, ctx, "/aircraft/"+url.PathEscape(icao24))
	if err != nil || info == nil {
		return nil, err
	}
	info.ICAO24 = icao24
	return info, nil
}
