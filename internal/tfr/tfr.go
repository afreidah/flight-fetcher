// -------------------------------------------------------------------------------
// TFR - Temporary Flight Restriction Feed
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Polls the FAA TFR JSON feed and caches the current list of active
// Temporary Flight Restrictions nationwide.
// -------------------------------------------------------------------------------

package tfr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	feedURL        = "https://tfr.faa.gov/tfr3/export/json"
	defaultTimeout = 15 * time.Second
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Restriction represents a single TFR from the FAA feed.
type Restriction struct {
	NotamID      string `json:"notam_id"`
	Type         string `json:"type"`
	Facility     string `json:"facility"`
	State        string `json:"state"`
	Description  string `json:"description"`
	CreationDate string `json:"creation_date"`
}

// Cache polls the FAA TFR feed and caches the results.
type Cache struct {
	httpClient *http.Client

	mu    sync.RWMutex
	items []Restriction
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewCache creates a TFR cache.
func NewCache() *Cache {
	return &Cache{
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Run polls the FAA TFR feed on the given interval. Blocks until ctx
// is cancelled.
func (c *Cache) Run(ctx context.Context, interval time.Duration) {
	c.refresh(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

// List returns the current cached TFR list.
func (c *Cache) List() []Restriction {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Restriction, len(c.items))
	copy(result, c.items)
	return result
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// refresh fetches the FAA TFR feed and updates the cache.
func (c *Cache) refresh(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		slog.WarnContext(ctx, "tfr: failed to create request", slog.String("error", err.Error()))
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "tfr: failed to fetch feed", slog.String("error", err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.WarnContext(ctx, "tfr: unexpected status", slog.Int("status", resp.StatusCode))
		return
	}

	var items []Restriction
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		slog.WarnContext(ctx, "tfr: failed to decode feed", slog.String("error", err.Error()))
		return
	}

	c.mu.Lock()
	c.items = items
	c.mu.Unlock()

	slog.InfoContext(ctx, "tfr: feed refreshed", slog.Int("count", len(items)))
}

// FormatType returns a display-friendly label for a TFR type.
func FormatType(t string) string {
	labels := map[string]string{
		"HAZARDS":              "Hazard",
		"VIP":                  "VIP",
		"SECURITY":             "Security",
		"SPACE OPERATIONS":     "Space Ops",
		"AIR SHOWS/SPORTS":     "Air Show",
		"SPECIAL":              "Special",
		"UAS PUBLIC GATHERING": "UAS/Drone",
	}
	if label, ok := labels[t]; ok {
		return label
	}
	return t
}

// TypeColor returns a CSS color for a TFR type.
func TypeColor(t string) string {
	colors := map[string]string{
		"HAZARDS":              "#c87533",
		"VIP":                  "#c44",
		"SECURITY":             "#c44",
		"SPACE OPERATIONS":     "#3a5a8c",
		"AIR SHOWS/SPORTS":     "#5b7a3a",
		"SPECIAL":              "#8e5a8c",
		"UAS PUBLIC GATHERING": "#8e7a3a",
	}
	if color, ok := colors[t]; ok {
		return color
	}
	return fmt.Sprintf("#%s", "888")
}
