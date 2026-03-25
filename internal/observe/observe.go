// -------------------------------------------------------------------------------
// Observe - OpenTelemetry and Prometheus Initialization
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Initializes the OpenTelemetry tracer provider with OTLP gRPC export and
// Prometheus metrics exporter. Provides a slog handler wrapper that injects
// trace_id and span_id into log records for correlation.
// -------------------------------------------------------------------------------

package observe

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	oteloprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// Setup initializes the global OTel tracer and meter providers. The tracer
// exports spans via OTLP gRPC (configurable via OTEL_EXPORTER_OTLP_ENDPOINT
// env var, defaults to localhost:4317). Metrics are exposed via Prometheus
// pull. Returns a shutdown function that flushes and closes exporters.
func Setup(ctx context.Context, serviceName, version string) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	// Traces — OTLP gRPC exporter (no-ops if no collector is running)
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metrics — Prometheus exporter
	promExporter, err := oteloprom.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}
	mp := metric.NewMeterProvider(
		metric.WithReader(promExporter),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	shutdown = func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down tracer: %w", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down meter: %w", err)
		}
		return nil
	}
	return shutdown, nil
}

// -------------------------------------------------------------------------
// SLOG TRACE HANDLER
// -------------------------------------------------------------------------

// TracedHandler wraps an slog.Handler to inject trace_id and span_id from
// the context into every log record, enabling log-trace correlation.
type TracedHandler struct {
	slog.Handler
}

// Handle adds trace context attributes before delegating to the wrapped handler.
func (h *TracedHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()))
	}
	if sc.HasSpanID() {
		r.AddAttrs(slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}
