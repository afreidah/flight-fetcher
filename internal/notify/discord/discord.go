// -------------------------------------------------------------------------------
// Discord - Webhook Notification Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Sends notifications to a Discord channel via webhook. Formats messages as
// rich embeds with structured fields.
// -------------------------------------------------------------------------------

package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/afreidah/flight-fetcher/internal/notify"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	defaultTimeout = 10 * time.Second
	embedColor     = 0xFF0000 // red
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client sends notifications to Discord via a webhook URL.
type Client struct {
	webhookURL string
	httpClient *http.Client
}

// webhook payload types matching the Discord API.
type webhookPayload struct {
	Embeds []embed `json:"embeds"`
}

type embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Color       int          `json:"color"`
	Fields      []embedField `json:"fields,omitempty"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Discord webhook client.
func New(webhookURL string) *Client {
	return &Client{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Send posts a notification message to Discord as a rich embed.
func (c *Client) Send(ctx context.Context, msg notify.Message) error {
	fields := make([]embedField, len(msg.Fields))
	for i, f := range msg.Fields {
		fields[i] = embedField{Name: f.Name, Value: f.Value, Inline: true}
	}

	payload := webhookPayload{
		Embeds: []embed{{
			Title:       msg.Title,
			Description: msg.Body,
			Color:       embedColor,
			Fields:      fields,
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending discord notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}
