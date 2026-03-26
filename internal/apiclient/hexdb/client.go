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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client communicates with the HexDB.io API.
type Client struct {
	*apiclient.Client
	imageBaseURL string
}

// hexdbResponse represents the HexDB.io API response format.
type hexdbResponse struct {
	Registration     string `json:"Registration"`
	ManufacturerName string `json:"ManufacturerName"`
	Type             string `json:"Type"`
	OperatorFlagCode string `json:"OperatorFlagCode"`
	ICAOTypeCode     string `json:"ICAOTypeCode"`
	RegisteredOwners string `json:"RegisteredOwners"`
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
		imageBaseURL: "https://hexdb.io",
	}
}

// Lookup fetches aircraft metadata by ICAO24 hex code. Returns nil if the
// aircraft is not found in HexDB.
func (c *Client) Lookup(ctx context.Context, icao24 string) (*aircraft.Info, error) {
	resp, err := apiclient.Lookup[hexdbResponse](c.Client, ctx, "/aircraft/"+url.PathEscape(icao24))
	if err != nil || resp == nil {
		return nil, err
	}

	imageURL := c.FetchImageURL(ctx, icao24)

	return &aircraft.Info{
		ICAO24:           icao24,
		Registration:     resp.Registration,
		ManufacturerName: resp.ManufacturerName,
		Type:             resp.Type,
		OperatorFlagCode: resp.OperatorFlagCode,
		ICAOTypeCode:     resp.ICAOTypeCode,
		RegisteredOwners: resp.RegisteredOwners,
		ImageURL:         imageURL,
	}, nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// FetchImageURL calls the HexDB image endpoint which returns the actual
// image URL as plain text. Returns empty string on any failure. Safe to
// call independently of the API client — uses DoRaw to bypass backoff.
func (c *Client) FetchImageURL(ctx context.Context, icao24 string) string {
	reqURL := fmt.Sprintf("%s/hex-image?hex=%s", c.imageBaseURL, url.QueryEscape(icao24))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return ""
	}
	resp, err := c.DoRaw(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return ""
	}
	imgURL := strings.TrimSpace(string(body))
	if !strings.HasPrefix(imgURL, "http") {
		return ""
	}
	return imgURL
}
