// -------------------------------------------------------------------------------
// Telegram - Bot API Notification Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Sends notifications to a Telegram chat via the Bot API. Formats messages
// with HTML markup for structured field display.
// -------------------------------------------------------------------------------

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/afreidah/flight-fetcher/internal/notify"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	defaultBaseURL = "https://api.telegram.org"
	defaultTimeout = 10 * time.Second
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client sends notifications to Telegram via the Bot API.
type Client struct {
	baseURL    string
	botToken   string
	chatID     string
	httpClient *http.Client
}

// sendMessageRequest is the Telegram Bot API sendMessage payload.
type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// apiResponse is the Telegram Bot API response envelope.
type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Telegram Bot API client.
func New(botToken, chatID string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		botToken:   botToken,
		chatID:     chatID,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Send posts a notification message to Telegram with HTML formatting.
func (c *Client) Send(ctx context.Context, msg notify.Message) error {
	text := formatHTML(msg)

	payload := sendMessageRequest{
		ChatID:    c.chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending telegram notification: %w", err)
	}
	defer resp.Body.Close()

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}
	return nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// formatHTML builds an HTML-formatted message for Telegram.
func formatHTML(msg notify.Message) string {
	var b strings.Builder
	b.WriteString("<b>")
	b.WriteString(msg.Title)
	b.WriteString("</b>\n")
	if msg.Body != "" {
		b.WriteString(msg.Body)
		b.WriteString("\n")
	}
	for _, f := range msg.Fields {
		b.WriteString("\n<b>")
		b.WriteString(f.Name)
		b.WriteString(":</b> ")
		b.WriteString(f.Value)
	}
	return b.String()
}
