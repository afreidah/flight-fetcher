// -------------------------------------------------------------------------------
// API Client - Shared HTTP Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Provides common HTTP client functionality for external API integrations:
// request construction with base URL, exponential backoff on 429 responses,
// response body size limiting, JSON decoding, and OpenTelemetry tracing and
// metrics. Embedded by each API-specific client.
// -------------------------------------------------------------------------------

package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	defaultTimeout = 10 * time.Second
	defaultMaxBody = 1 * 1024 * 1024 // 1MB
	initialBackoff = 30 * time.Second
	maxBackoff     = 10 * time.Minute
	backoffFactor  = 2
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Options configures a new Client.
type Options struct {
	Name         string // identifies this client in spans and metrics
	BaseURL      string
	Timeout      time.Duration
	MaxBodyBytes int64
}

// Client provides shared HTTP functionality for external API integrations.
// Handles exponential backoff on rate limits, response body size limiting,
// JSON decoding, and OTel instrumentation. Intended to be embedded by
// API-specific clients.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	maxBodyBytes int64
	name         string

	tracer    trace.Tracer
	reqCount  metric.Int64Counter
	reqDur    metric.Float64Histogram

	mu          sync.Mutex
	backoff     time.Duration
	backoffUtil time.Time
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Client with the given options. Applies sensible defaults
// for timeout (10s) and max body size (1MB) when not specified.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultMaxBody
	}
	if opts.Name == "" {
		opts.Name = "apiclient"
	}

	meter := otel.Meter("flight-fetcher/apiclient")
	reqCount, _ := meter.Int64Counter("apiclient.requests",
		metric.WithDescription("Total API requests by upstream and status"),
	)
	reqDur, _ := meter.Float64Histogram("apiclient.request.duration",
		metric.WithDescription("API request duration in seconds"),
		metric.WithUnit("s"),
	)

	return &Client{
		httpClient:   &http.Client{Timeout: opts.Timeout},
		baseURL:      opts.BaseURL,
		maxBodyBytes: opts.MaxBodyBytes,
		name:         opts.Name,
		tracer:       otel.Tracer("flight-fetcher/apiclient"),
		reqCount:     reqCount,
		reqDur:       reqDur,
		backoff:      initialBackoff,
	}
}

// NewRequest creates an HTTP request with the base URL prepended to path.
// Returns an error if the client is in a backoff period.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if err := c.checkBackoff(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	return req, nil
}

// Do executes a request with circuit breaker behavior and OTel
// instrumentation. Applies exponential backoff on rate limits (429),
// server errors (5xx), and transport failures. The caller must close
// resp.Body on success.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	ctx, span := c.tracer.Start(req.Context(), c.name+".request",
		trace.WithAttributes(
			attribute.String("http.method", req.Method),
			attribute.String("http.url", req.URL.Path),
			attribute.String("upstream", c.name),
		),
	)
	defer span.End()

	start := time.Now()
	attrs := []attribute.KeyValue{attribute.String("upstream", c.name)}
	defer func() {
		c.reqCount.Add(ctx, 1, metric.WithAttributes(attrs...))
		c.reqDur.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	}()

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "transport error")
		attrs = append(attrs, attribute.String("status", "error"))
		c.applyBackoff(ctx)
		return nil, fmt.Errorf("executing request: %w", err)
	}

	attrs = append(attrs, attribute.String("status", strconv.Itoa(resp.StatusCode)))
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		span.SetStatus(codes.Error, "rate limited")
		c.applyBackoff(ctx)
		return nil, fmt.Errorf("rate limited (429), backing off for %s", c.currentBackoff())
	}
	if resp.StatusCode >= 500 {
		resp.Body.Close()
		span.SetStatus(codes.Error, "server error")
		c.applyBackoff(ctx)
		return nil, fmt.Errorf("server error (%d), backing off for %s", resp.StatusCode, c.currentBackoff())
	}

	c.resetBackoff()
	return resp, nil
}

// DoRaw executes a request without backoff checking or handling. Used for
// auxiliary requests like OAuth2 token endpoints that should not participate
// in the main API's backoff state.
func (c *Client) DoRaw(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// DecodeJSON reads from r with the configured body size limit and decodes
// JSON into v.
func (c *Client) DecodeJSON(r io.Reader, v any) error {
	if err := json.NewDecoder(io.LimitReader(r, c.maxBodyBytes)).Decode(v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// RequestOption modifies a request before it is sent. Used to set
// authentication headers or other per-request configuration.
type RequestOption func(*http.Request)

// Lookup performs a GET request to path, decodes the JSON response into T,
// and returns the result. Returns (nil, nil) on 404. Any RequestOption
// callbacks are applied to the request before sending (e.g. for auth headers).
func Lookup[T any](c *Client, ctx context.Context, path string, opts ...RequestOption) (*T, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(req)
	}

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

	var result T
	if err := c.DecodeJSON(resp.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// checkBackoff returns an error if the client is currently in a backoff
// period, skipping the request entirely.
func (c *Client) checkBackoff(ctx context.Context) error {
	c.mu.Lock()
	until := c.backoffUtil
	c.mu.Unlock()

	if time.Now().Before(until) {
		remaining := time.Until(until).Truncate(time.Second)
		slog.InfoContext(ctx, "skipping request during backoff",
			slog.String("remaining", remaining.String()))
		return fmt.Errorf("rate limited, backoff expires in %s", remaining)
	}
	return nil
}

// applyBackoff doubles the backoff duration (up to maxBackoff) and sets the
// next allowed request time.
func (c *Client) applyBackoff(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.backoffUtil = time.Now().Add(c.backoff)
	slog.WarnContext(ctx, "rate limited, applying backoff",
		slog.String("duration", c.backoff.String()))

	c.backoff *= backoffFactor
	if c.backoff > maxBackoff {
		c.backoff = maxBackoff
	}
}

// resetBackoff clears the backoff state after a successful request.
func (c *Client) resetBackoff() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.backoff = initialBackoff
	c.backoffUtil = time.Time{}
}

// currentBackoff returns the current backoff duration for logging.
func (c *Client) currentBackoff() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.backoff
}
