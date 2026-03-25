// -------------------------------------------------------------------------------
// Observe - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests OTel setup, shutdown, and slog trace context injection.
// -------------------------------------------------------------------------------

package observe

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// TestSetup_ReturnsShutdown verifies that Setup initializes providers and
// returns a working shutdown function.
func TestSetup_ReturnsShutdown(t *testing.T) {
	ctx := context.Background()
	shutdown, err := Setup(ctx, "test-service", "v0.0.1")
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if shutdown == nil {
		t.Fatal("Setup() returned nil shutdown")
	}

	// Verify tracer provider was set
	tp := otel.GetTracerProvider()
	if tp == nil {
		t.Fatal("tracer provider not set")
	}

	// Verify meter provider was set
	mp := otel.GetMeterProvider()
	if mp == nil {
		t.Fatal("meter provider not set")
	}

	// Shutdown should not error
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown() error = %v", err)
	}
}

// TestTracedHandler_WithActiveSpan verifies that trace_id and span_id are
// injected into log records when a span is active.
func TestTracedHandler_WithActiveSpan(t *testing.T) {
	ctx := context.Background()
	shutdown, err := Setup(ctx, "test-service", "v0.0.1")
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	var buf bytes.Buffer
	handler := &TracedHandler{
		Handler: slog.NewJSONHandler(&buf, nil),
	}
	logger := slog.New(handler)

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(ctx, "test-span")
	defer span.End()

	logger.InfoContext(ctx, "test message")

	output := buf.String()
	sc := trace.SpanContextFromContext(ctx)
	if !strings.Contains(output, sc.TraceID().String()) {
		t.Errorf("log output missing trace_id: %s", output)
	}
	if !strings.Contains(output, sc.SpanID().String()) {
		t.Errorf("log output missing span_id: %s", output)
	}
}

// TestTracedHandler_WithoutSpan verifies that no trace attributes are added
// when no span is active.
func TestTracedHandler_WithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	handler := &TracedHandler{
		Handler: slog.NewJSONHandler(&buf, nil),
	}
	logger := slog.New(handler)

	logger.InfoContext(context.Background(), "no span")

	output := buf.String()
	if strings.Contains(output, "trace_id") {
		t.Errorf("log output should not contain trace_id without active span: %s", output)
	}
	if strings.Contains(output, "span_id") {
		t.Errorf("log output should not contain span_id without active span: %s", output)
	}
}
