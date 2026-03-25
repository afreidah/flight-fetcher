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

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the HexDB.io API.
type Client struct {
	*apiclient.Client
}

// hexdbResponse represents the HexDB.io API response format.
type hexdbResponse struct {
	Registration     string `json:"Registration"`
	ManufacturerName string `json:"ManufacturerName"`
	Type             string `json:"Type"`
	OperatorFlagCode string `json:"OperatorFlagCode"`
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
func (c *Client) Lookup(ctx context.Context, icao24 string) (*aircraft.Info, error) {
	resp, err := apiclient.Lookup[hexdbResponse](c.Client, ctx, "/aircraft/"+url.PathEscape(icao24))
	if err != nil || resp == nil {
		return nil, err
	}
	return &aircraft.Info{
		ICAO24:           icao24,
		Registration:     resp.Registration,
		ManufacturerName: resp.ManufacturerName,
		Type:             resp.Type,
		OperatorFlagCode: resp.OperatorFlagCode,
	}, nil
}
