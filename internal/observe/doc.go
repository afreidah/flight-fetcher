// -------------------------------------------------------------------------------
// Observe - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the observe package.
// -------------------------------------------------------------------------------

// Package observe wires up the three observability signals and connects them to
// each other: a tracer provider exporting spans over OTLP gRPC, a Prometheus
// metrics exporter scraped from the dashboard's /metrics endpoint, and a slog
// handler wrapper that stamps trace_id and span_id onto every record.
//
// The handler wrapper is the part that makes the other two useful. Because
// every log call in this codebase takes a context, a line emitted inside a span
// carries that span's identifiers, so a trace can be pivoted to its logs and
// back without correlating on timestamps.
//
// Setup is deliberately tolerant of a missing collector: the OTLP exporter
// no-ops rather than failing, so the service runs unchanged in a development
// environment with no tracing backend. It returns a shutdown function that
// flushes pending spans, which the entrypoint must defer or the last spans
// before exit are lost.
//
// observe is a leaf package. It imports nothing from this module and is
// initialized once by the composition root before any other component starts.
package observe
