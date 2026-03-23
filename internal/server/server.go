// -------------------------------------------------------------------------------
// Server - Web Dashboard HTTP Server
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Serves a lightweight web dashboard for viewing current flight state from Redis
// and enriched aircraft metadata from Postgres. Provides JSON API endpoints and
// an embedded HTML frontend.
// -------------------------------------------------------------------------------

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/afreidah/flight-fetcher/internal/airlabs"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/opensky"
)

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// FlightLister returns all current flights from the cache.
type FlightLister interface {
	GetAllFlights(ctx context.Context) ([]opensky.StateVector, error)
	GetFlight(ctx context.Context, icao24 string) (*opensky.StateVector, error)
}

// AircraftMetaReader retrieves cached aircraft metadata by ICAO24.
type AircraftMetaReader interface {
	GetAircraftMeta(ctx context.Context, icao24 string) (*hexdb.AircraftInfo, error)
}

// RouteReader retrieves cached flight route information by callsign.
type RouteReader interface {
	GetFlightRoute(ctx context.Context, callsign string) (*airlabs.FlightRoute, error)
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Server serves the web dashboard and flight data API.
type Server struct {
	flights  FlightLister
	aircraft AircraftMetaReader
	routes   RouteReader
	mux      *http.ServeMux
}

// flightDetail combines live state, enriched metadata, and route information.
type flightDetail struct {
	State    *opensky.StateVector `json:"state"`
	Aircraft *hexdb.AircraftInfo  `json:"aircraft,omitempty"`
	Route    *airlabs.FlightRoute `json:"route,omitempty"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Server with the given data sources.
func New(flights FlightLister, aircraft AircraftMetaReader, routes RouteReader) *Server {
	s := &Server{
		flights:  flights,
		aircraft: aircraft,
		routes:   routes,
		mux:      http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /api/flights", s.handleListFlights)
	s.mux.HandleFunc("GET /api/flights/{icao24}", s.handleGetFlight)
	return s
}

// ListenAndServe starts the HTTP server on the given address. Blocks until
// the context is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context, addr string) {
	srv := &http.Server{Addr: addr, Handler: s.mux}

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "dashboard shutting down")
		srv.Close()
	}()

	slog.InfoContext(ctx, "dashboard listening", slog.String("addr", addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.ErrorContext(ctx, "dashboard error", slog.String("error", err.Error()))
	}
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// handleIndex serves the embedded HTML dashboard page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(indexHTML); err != nil {
		slog.WarnContext(r.Context(), "failed to write index page",
			slog.String("error", err.Error()))
	}
}

// handleListFlights returns all current flights as JSON.
func (s *Server) handleListFlights(w http.ResponseWriter, r *http.Request) {
	flights, err := s.flights.GetAllFlights(r.Context())
	if err != nil {
		http.Error(w, "failed to list flights", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: list flights failed",
			slog.String("error", err.Error()))
		return
	}
	writeJSON(r.Context(), w, flights)
}

// handleGetFlight returns live state and enriched metadata for a single aircraft.
func (s *Server) handleGetFlight(w http.ResponseWriter, r *http.Request) {
	icao24 := r.PathValue("icao24")

	sv, err := s.flights.GetFlight(r.Context(), icao24)
	if err != nil {
		http.Error(w, "failed to get flight", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: get flight failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return
	}
	if sv == nil {
		http.Error(w, "flight not found", http.StatusNotFound)
		return
	}

	detail := flightDetail{State: sv}
	meta, err := s.aircraft.GetAircraftMeta(r.Context(), icao24)
	if err != nil {
		slog.WarnContext(r.Context(), "api: aircraft meta lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
	detail.Aircraft = meta

	if s.routes != nil && sv.Callsign != "" {
		route, err := s.routes.GetFlightRoute(r.Context(), strings.TrimSpace(sv.Callsign))
		if err != nil {
			slog.WarnContext(r.Context(), "api: route lookup failed",
				slog.String("icao24", icao24),
				slog.String("error", err.Error()))
		}
		detail.Route = route
	}

	writeJSON(r.Context(), w, detail)
}

// writeJSON encodes v as JSON and writes it to w.
func writeJSON(ctx context.Context, w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.WarnContext(ctx, "failed to write JSON response",
			slog.String("error", err.Error()))
	}
}
