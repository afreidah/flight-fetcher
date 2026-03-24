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
	"time"

	"github.com/afreidah/flight-fetcher/internal/airlabs"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/squawk"
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

// SquawkAlertReader retrieves recent emergency squawk alerts.
type SquawkAlertReader interface {
	GetRecentSquawkAlerts(ctx context.Context, since time.Duration) ([]squawk.Alert, error)
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Server serves the web dashboard and flight data API.
type Server struct {
	flights  FlightLister
	aircraft AircraftMetaReader
	routes   RouteReader
	alerts   SquawkAlertReader
	version  string
	refresh  int
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

// New creates a Server with the given data sources, version string, and
// dashboard refresh interval in seconds.
func New(flights FlightLister, aircraft AircraftMetaReader, routes RouteReader, alerts SquawkAlertReader, version string, refreshSec int) *Server {
	s := &Server{
		flights:  flights,
		aircraft: aircraft,
		routes:   routes,
		alerts:   alerts,
		version:  version,
		refresh:  refreshSec,
		mux:      http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /api/flights", s.handleListFlights)
	s.mux.HandleFunc("GET /api/flights/{icao24}", s.handleGetFlight)
	s.mux.HandleFunc("GET /api/squawk-alerts", s.handleSquawkAlerts)
	s.mux.HandleFunc("GET /api/aircraft/{icao24}", s.handleGetAircraft)
	s.mux.HandleFunc("GET /api/routes/{callsign}", s.handleGetRoute)
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
	if _, err := w.Write(renderedHTML(s.version, s.refresh)); err != nil {
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
	if meta != nil && meta.IsSentinel() {
		meta = nil
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

// handleSquawkAlerts returns recent emergency squawk alerts as JSON.
func (s *Server) handleSquawkAlerts(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil {
		writeJSON(r.Context(), w, []any{})
		return
	}
	alerts, err := s.alerts.GetRecentSquawkAlerts(r.Context(), 24*time.Hour)
	if err != nil {
		http.Error(w, "failed to get squawk alerts", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: squawk alerts failed",
			slog.String("error", err.Error()))
		return
	}
	writeJSON(r.Context(), w, alerts)
}

// handleGetAircraft returns cached aircraft metadata by ICAO24.
func (s *Server) handleGetAircraft(w http.ResponseWriter, r *http.Request) {
	icao24 := r.PathValue("icao24")
	meta, err := s.aircraft.GetAircraftMeta(r.Context(), icao24)
	if err != nil {
		http.Error(w, "failed to get aircraft", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: aircraft lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return
	}
	if meta == nil || meta.IsSentinel() {
		http.Error(w, "aircraft not found", http.StatusNotFound)
		return
	}
	writeJSON(r.Context(), w, meta)
}

// handleGetRoute returns cached flight route information by callsign.
func (s *Server) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	callsign := r.PathValue("callsign")
	if s.routes == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	route, err := s.routes.GetFlightRoute(r.Context(), callsign)
	if err != nil {
		http.Error(w, "failed to get route", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: route lookup failed",
			slog.String("callsign", callsign),
			slog.String("error", err.Error()))
		return
	}
	if route == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	writeJSON(r.Context(), w, route)
}

// writeJSON encodes v as JSON and writes it to w.
func writeJSON(ctx context.Context, w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.WarnContext(ctx, "failed to write JSON response",
			slog.String("error", err.Error()))
	}
}
